# Contracts: rbac-manager

Stable, machine-checkable behavior for this example. See
[EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for the format.

### `./setup.sh` (real use)

- mutates: yes (ConfigHub only)
- creates: 3 Warn=true guardrail Triggers (label `Pack=rbac-guardrails`) plus a
  `Trigger` Filter selecting them, ONCE in a policy Space (default
  `policy-guardrails`, override with `--policy-space SLUG`); and the 4
  parameterized set-yq edit Invocations (`rbac-add-verb`, `rbac-remove-verb`,
  `rbac-add-subject`, `rbac-remove-subject`) in the `rbac-edits` Space — the
  shared, declarative edit templates the app and the agent CLI invoke with
  parameters instead of compiling yq client-side
- wires: points each in-scope Space's `TriggerFilterID` at that Filter —
  Spaces with Kubernetes/YAML Units, optionally narrowed with `--where-space EXPR`
- skips: Spaces with a custom `WhereTrigger`, a different `TriggerFilterID`, or
  Triggers of their own (reported, not modified); already-wired Spaces and
  existing objects (idempotent)
- supports: `--policy-space SLUG`, `--where-space EXPR`, `--explain` /
  `--explain-json` (the latter two mutate nothing)
- proves: guardrail validation can be installed on a real organization, defined
  once and enforced fleet-wide, without blocking anyone (ApplyWarnings, not
  ApplyGates)

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
- stable text anchors: `rbac-manager setup plan`, `Mutates: ConfigHub only.`
- proves: the example plan before any mutation

### `./demo-setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`,
  `mutates_live_infra`, `spaces`, `units`, `notes.expected_apply_gates`,
  `evaluation_modes`
- proves: the example plan, including which planted violations must end up
  gated, in machine-readable form

### `./demo-setup.sh`

- mutates: yes (ConfigHub only; no Targets, Workers, or live infrastructure)
- creates: 5 spaces, 5 triggers, 2 filters, 4 base units, 12 cloned units,
  3 violation units, 1 divergence revision; plus the `rbac-edits` Space and its
  4 parameterized set-yq edit Invocations (shared with the web app and agent CLI)
- idempotent: re-running skips existing entities (`exists, skipping`)
- cleanup: none by design (demo data persists); manual teardown documented in
  AI_START_HERE.md

### `./demo-verify.sh`

- mutates: no
- output shape: plain text, one `ok`/`FAIL` line per check
- stable success text: `All checks passed.`
- proves: the Space/Trigger/Filter/Unit layout exists; each planted violation
  carries exactly its intended Apply Gate; the orphaned binding carries no
  gate; prod requires approval; clean personas are ungated; dev diverges from
  base and staging does not

### `cub unit get legacy-wildcard-admin --space rbac-demo-dev -o jq=".Unit.ApplyGates"`

- mutates: no
- output shape: JSON object keyed by gate name
- stable fields: key `rbac-demo-policy/no-wildcards/vet-celexpr` with value `true`
- proves: guardrail policies are enforced as Apply Gates, not advisory lint

### `cub function do --space "*" --where "Labels.persona = 'developer' AND Labels.env = 'staging'" --change-desc "..." -- yq-i '...'`

- mutates: yes (ConfigHub)
- output shape: plain text per-unit success + persisted revision with the
  change description
- proves: one selector-scoped fleet edit replaces the per-cluster GitOps
  base/patch/overlay ceremony; the mutation runs server-side and preserves
  YAML comments and formatting
