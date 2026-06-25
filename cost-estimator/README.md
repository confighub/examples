# cost-estimator

Workload cloud cost managed as **data**, in ConfigHub.

A custom estimator reads each workload's resource requests from its ConfigHub
Unit, costs them against a static, versioned price book (CPU + memory + storage),
and writes the monthly estimate **and a budget verdict** back onto the Unit as
annotations. Guardrail Triggers then gate apply on whatever it flagged OVER
budget — so a cost overrun is caught at config time, not on the cloud bill. The
config (the resource requests) and the verdict (the cost) are the same versioned
object.

This is the same shape as the [`sec-scanner`](../sec-scanner) (CVEs) and
[`rbac-manager`](../rbac-manager) examples — analyze the fleet, record the
verdict as data, gate on it — applied to cost.

```
  policy Space                 base Space
  (guardrail Triggers            (workload Units with resource requests:
   + Filters, no Units)           frontend, api, cache, db)
        |                              |  clone (upstream/downstream)
        | TriggerFilterID              v
        +----------->  cost-demo-dev     (Environment=Dev;     + planted violations)
        +----------->  cost-demo-staging (Environment=Staging)
        +----------->  cost-demo-prod    (Environment=Prod;    + approval required)

   pricebook.json (CPU/mem/storage rates + per-env budgets)
        ▲
        │ cost = requests × replicas × rates  (+ storage)
   estimator ── read requests ─▶ cost ─▶ budget verdict ─▶ write back to Units
```

## Guardrails

Defined once in a policy Space, enforced fleet-wide via a Trigger Filter:

| Trigger | Function | Blocks |
|---|---|---|
| `valid-schemas` | `vet-schemas` | malformed Kubernetes resources |
| `requests-required` | `vet-celexpr` | workloads with no cpu/memory requests (uncostable) |
| `within-budget` | `vet-celexpr` | workloads the estimator flagged `budget-status=OVER` |

`within-budget` gates on a `cost-estimator.confighub.com/budget-status`
annotation the estimator writes — exactly as sec-scanner's `no-critical-cves`
gates on a `max-severity` annotation. The verdict is a discrete `OK` / `WARN` /
`OVER`, so the gate is a clean string compare; the numeric `monthly-usd` rides
along as informational data.

## The estimator (`estimator/`)

A self-contained Go binary (`costest`) that talks to the ConfigHub REST API
directly (the same surface the web app uses) — no `cub` shell-out:

```bash
costest estimate <file|->            # cost one manifest against the price book
costest inventory --space <glob>     # the fleet's cost inventory, one query
costest estimate-fleet --space <glob> --write-back   # cost every workload; record as data
```

The cost model and budgets live in [`pricing/pricebook.json`](./pricing/) — a
static, versioned, editable file. Nothing here calls a live cloud-pricing API;
the point is a deterministic, auditable estimate.

## The console (`app/`)

A static React + MUI SPA that reads the estimator's annotations and the
guardrail gates from the ConfigHub API and shows the fleet's spend — total
monthly cost, the breakdown by environment, budget-status counts, top spenders,
and a per-workload table with budget + gate chips. It computes nothing itself.
Built on a generated `openapi-fetch` client (`app/src/sdk/`, refreshed with
`npm run vendor-sdk`).

## Run it

```bash
./demo-setup.sh --explain        # preview the plan, mutate nothing
./demo-setup.sh                  # seed the cost-demo fleet + estimate + gate
./demo-verify.sh                 # confirm the layout, gate matrix, and estimates
```

To install the guardrails on your **own** fleet (no demo data):

```bash
./setup.sh --explain                                 # preview
./setup.sh --where-space "Labels.Environment = 'Prod'"   # install, scoped
./estimator/costest estimate-fleet --space "<your-spaces>" --write-back
```

See [`AI_START_HERE.md`](./AI_START_HERE.md) for a paced, stage-by-stage
walkthrough and [`contracts.md`](./contracts.md) for the machine-checkable
command behavior.

## Boundaries

- **Paper clusters.** ConfigHub Spaces only — no Targets, Workers, or live
  deploys, and no external network.
- **Static pricing.** The price book is a checked-in JSON, not a billing API.
  CPU + memory + storage only (no GPU / network / egress in v1).
- **The estimator computes; ConfigHub stores.** The binary holds no state; every
  durable thing (requests, estimate, verdict, budgets) is ConfigHub or the price
  book.
