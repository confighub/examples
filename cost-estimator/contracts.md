# Contracts: cost-estimator

Stable, machine-checkable behavior for this example. See
[EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for the format.

### `./setup.sh` (real use)

- mutates: yes (ConfigHub only)
- creates: 3 Warn=true budget guardrail Triggers (label `Pack=cost-guardrails`)
  plus a `Trigger` Filter selecting them, ONCE in a policy Space (default
  `policy-guardrails`, override with `--policy-space SLUG`)
- wires: points each in-scope Space's `TriggerFilterID` at that Filter —
  Spaces with Kubernetes/YAML Units, optionally narrowed with `--where-space EXPR`
- skips: Spaces with a custom `WhereTrigger`, a different `TriggerFilterID`, or
  Triggers of their own (reported, not modified); already-wired Spaces and
  existing objects (idempotent)
- supports: `--policy-space SLUG`, `--where-space EXPR`, `--explain` /
  `--explain-json` (the latter two mutate nothing)
- proves: budget guardrails can be installed on a real organization, defined
  once and enforced fleet-wide, without blocking anyone (ApplyWarnings)

### `./verify.sh` (real use)

- mutates: no
- supports: `--policy-space SLUG`, `--where-space EXPR`
- output shape: plain text, one `ok`/`FAIL` line per check
- stable success text: `All checks passed.`
- proves: the policy Space holds the three guardrail Triggers (warn or promoted
  to blocking) and the Filter, and every in-scope Space points its
  `TriggerFilterID` at that Filter

### `./demo-setup.sh --explain`

- mutates: no
- output shape: plain text plan with ASCII diagram
- stable text anchors: `cost-estimator setup plan`, `Mutates: ConfigHub`
- proves: the example plan before any mutation

### `./demo-setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`,
  `mutates_live_infra`, `spaces`, `units`, `pricing_model`,
  `notes.expected_apply_gates`, `evaluation_modes`
- proves: the plan in machine-readable form, including which planted violations
  must end up gated and by which guardrail

### `./demo-setup.sh`

- mutates: yes (ConfigHub Spaces/Units only; no Targets, Workers, or live
  infrastructure; no external network)
- creates: 5 spaces, 4 triggers, 2 filters, 4 base units, 12 cloned units,
  2 violation units; then runs the estimator, writes the cost + budget-verdict
  annotations back onto every workload (incl. `estimated-at` + `pricing-version`),
  publishes one `AppConfig/YAML` `cost-estimate-record` Unit per Space (the full
  per-workload estimates), and a `pricebook-status` Unit (current price-book
  version) in the policy Space
- supports: `--no-estimate` (seed ConfigHub only; skip the estimate)
- idempotent: re-running skips existing entities and re-estimates
- cleanup: none by design (demo data persists); manual teardown in AI_START_HERE.md

### `./demo-verify.sh`

- mutates: no
- output shape: plain text, one `ok`/`FAIL` line per check
- stable success text: `All checks passed.`
- proves: the Space/Trigger/Filter/Unit layout exists; the estimator wrote
  `budget-status=OVER` onto the over-provisioned Unit and `monthly-usd` onto
  every workload; each planted violation carries exactly its intended Apply Gate
  (`oversized-analytics` → `within-budget`, `no-requests-web` →
  `requests-required`); clean workloads are ungated; prod requires approval;
  each Space's `AppConfig/YAML` `cost-estimate-record` Unit exists and holds the
  per-workload estimates; units record the `pricing-version` they were costed
  against and the `pricebook-status` Unit is present; the price book is present

### `./estimator/costest estimate <file|->`

- mutates: no
- reads: a Kubernetes manifest (file or stdin) + the price book
  (`--pricebook`, default `pricing/pricebook.json`)
- output shape: a JSON cost report — `{kind, name, replicas, cpu_cores,
  memory_gb, storage_gb, region, environment, monthly_usd, budget_usd,
  budget_status, missing_requests, pricing_version}`
- supports: `--region R`, `--env E`, `--budgets FILE`
- proves: one workload's monthly cost and budget verdict are computed
  deterministically from its resource requests and a static price book

### `./estimator/costest inventory --space <glob>`

- mutates: no
- reads: the ConfigHub REST API directly (`CONFIGHUB_URL` + `CONFIGHUB_TOKEN`),
  the same surface the web app uses — no `cub` CLI
- output shape: plain text `SPACE UNIT KIND CPU MEM STG MONTHLY BUDGET` rows, or
  JSON (`--json`)
- proves: the fleet's cost inventory is a single query over ConfigHub data

### `./estimator/costest estimate-fleet --space <glob> --write-back`

- mutates: yes (ConfigHub) — on each workload Unit, sets the gate-signal +
  provenance annotations `cost-estimator.confighub.com/budget-status`,
  `.../monthly-usd`, `.../cpu-cores`, `.../memory-gb`, `.../storage-gb`,
  `.../estimated-at`, `.../pricing-version` via a server-side `yq-i` invocation;
  and upserts one **`AppConfig/YAML`** `cost-estimate-record` Unit per Space
  (labeled `role=cost-record`) holding the full per-workload estimates
- supports: `--status-space SLUG` (upsert a `pricebook-status` `AppConfig/YAML`
  Unit recording the current price-book version), `--budgets FILE`,
  `--where EXPR`, `--fail-on-over` (exit 3 if any workload is OVER), `--json`
- all via the REST API; requires `CONFIGHUB_URL` + `CONFIGHUB_TOKEN`
- proves: cost estimates become ConfigHub data, which the within-budget
  guardrail then gates on — config and verdict are the same versioned object
