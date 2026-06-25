# AI Guide: cost-estimator

Workload cloud cost managed as data: a static, versioned price book, a custom
estimator that costs each workload from its resource requests, cost estimates +
budget verdicts written back onto ConfigHub Units, and Apply Gates that block
the over-budget.

## CRITICAL: Demo Pacing

Run ONE stage at a time. After each stage, STOP and wait for the human before
continuing. Do not run ahead, even if the next command is obvious. The human
may want to inspect the GUI or the estimates between stages.

## Suggested Prompt

> Walk me through the cost-estimator example stage by stage. Pause after each
> stage so I can look around. I'm authenticated with cub. (No cluster needed —
> these are paper clusters; nothing deploys.)

## Stage 1: Cost one workload

```bash
( cd estimator && go build -o costest . )            # build the estimator

# cost a small Deployment, then a deliberately over-provisioned one (dev budget)
./estimator/costest estimate --env prod manifests/workloads/db.yaml
./estimator/costest estimate --env dev  manifests/violations/oversized-analytics.yaml
```

Expected: `db` costs a few dollars a month (note the storage line from its
volumeClaimTemplates); `oversized-analytics` is well over a thousand a month and
reports `"budget_status": "OVER"`. The estimate is `requests × replicas × price
book` — deterministic, offline, auditable. `--json` is the default shape.

**PAUSE.** Wait for the human.

## Stage 2: Seed the fleet and gate it

```bash
./demo-setup.sh --explain        # preview first — mutates nothing
./demo-setup.sh                  # seed Spaces/Units, then estimate + write back + gate
./demo-verify.sh                 # layout + gate matrix + estimate write-back
```

Expected: `All checks passed.` Re-running demo-setup.sh is safe; it skips
existing entities and re-estimates.

GUI gap: there is no single fleet-level view of "what each workload costs, where
the money goes, and what's over budget" — that is what the cost-estimator
console webapp adds on top of this layout (as rbac-manager's webapp does for
RBAC).

GUI feature ask: a fleet cost dashboard that summarizes, across a
label-selected set of Spaces, total monthly spend, the breakdown by environment,
each Unit's CPU/memory/storage and monthly estimate, its budget status, and
whether it is currently gated — with a drill-down to the per-workload cost
inputs the estimator recorded.

**PAUSE.** Wait for the human.

## Stage 3: The fleet cost inventory

The complaint this answers: costing one workload is easy; knowing what the whole
fleet costs and which workloads blow their budget is the hard part.

```bash
# inventory/estimate-fleet talk to the ConfigHub REST API directly (like the web
# app), so give the estimator a token first:
export CONFIGHUB_URL="https://hub.confighub.com" CONFIGHUB_TOKEN="$(cub auth get-token)"

# every workload, every environment, one query — with its monthly cost + verdict
./estimator/costest inventory --space "cost-demo-*" --pricebook pricing/pricebook.json

# which Units did the estimator mark OVER budget? (verdict stored as data)
cub unit list --space "*" --where "Annotations.'cost-estimator.confighub.com/budget-status' = 'OVER'"
```

Expected: the inventory lists each Unit's cost and budget status; the query
returns the dev `oversized-analytics` Unit the estimator flagged.

**PAUSE.** Wait for the human.

## Stage 4: Guardrails are gates, not advice

```bash
# the over-provisioned workload is blocked from ever being applied
cub unit get oversized-analytics --space cost-demo-dev -o jq=".Unit.ApplyGates"

# so is the workload with no resource requests (static check, no estimate needed)
cub unit get no-requests-web --space cost-demo-dev -o jq=".Unit.ApplyGates"

# prod changes carry an approval gate out of the box
cub unit get frontend --space cost-demo-prod -o jq=".Unit.ApplyGates"
```

Expected gates: `cost-demo-policy/within-budget/vet-celexpr`,
`cost-demo-policy/requests-required/vet-celexpr`, and
`cost-demo-policy/require-approval/vet-approvedby` respectively. The clean
workloads within budget carry NO gate.

**PAUSE.** Wait for the human.

## Stage 5: Right-size, re-estimate, gate clears

```bash
# cut the over-provisioned workload down to 2 replicas (server-side, comment-preserving)
cub function do --space cost-demo-dev --unit oversized-analytics \
  --change-desc "Right-size analytics to fit the dev budget" \
  -- yq-i '.spec.replicas = 1'

# re-estimate + write back; the OVER verdict flips and the gate lifts
./estimator/costest estimate-fleet --space "cost-demo-dev" --write-back --pricebook pricing/pricebook.json
cub unit get oversized-analytics --space cost-demo-dev -o jq=".Unit.ApplyGates"
```

Expected: after the right-size and re-estimate, `oversized-analytics` is no
longer gated by within-budget — the fix and the verdict are the same versioned
object. (One replica of 4cpu/16Gi is still pricey; drop requests too if it
remains OVER.)

**PAUSE.** Wait for the human.

## Stage 6: Re-price the fleet — know what changed

Cloud prices change. Each estimate records the price-book version it ran against
(`pricing-version`), and the policy Space holds the current `pricebook-status`,
so after you edit `pricing/pricebook.json` you re-estimate to re-stamp the fleet.

```bash
# raise the CPU rate in pricing/pricebook.json (and bump "version"), then:
./estimator/costest estimate-fleet --space "cost-demo-*" --write-back --status-space cost-demo-policy --pricebook pricing/pricebook.json
cub unit data --space cost-demo-policy pricebook-status     # the version the fleet was costed against
```

Expected: every workload's `monthly-usd` and `pricing-version` update in one
pass; `pricebook-status` reflects the new version.

**PAUSE.** Wait for the human.

## Stage 7: The console (optional)

A static React SPA renders all of the above from the ConfigHub API — no extra
backend. It reads the estimator's annotations + the guardrail gates; it computes
nothing itself.

```bash
cd app && npm install
CONFIGHUB_URL=https://hub.confighub.com npm run dev    # then open http://localhost:5193
```

Paste a token from `cub auth get-token` when prompted (dev mode). The Dashboard
shows total monthly spend, the breakdown by environment, budget-status counts,
and top spenders; Fleet is the per-workload cost table with a budget-status chip
and gate chips; clicking a row opens the cost breakdown for that workload.

**PAUSE.** Wait for the human.

## Cleanup

No cleanup script by design (demo data is meant to persist). To tear down:

```bash
for s in cost-demo-dev cost-demo-staging cost-demo-prod cost-demo-base cost-demo-policy; do
  cub space delete "$s" --recursive
done
```
