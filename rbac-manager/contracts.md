# Contracts: rbac-manager

Stable, machine-checkable behavior for this example. See
[EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for the format.

### `./setup.sh --explain`

- mutates: no
- output shape: plain text plan with ASCII diagram
- stable text anchors: `rbac-manager setup plan`, `Mutates: ConfigHub only.`
- proves: the example plan before any mutation

### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`,
  `mutates_live_infra`, `spaces`, `units`, `notes.expected_apply_gates`,
  `evaluation_modes`
- proves: the example plan, including which planted violations must end up
  gated, in machine-readable form

### `./setup.sh`

- mutates: yes (ConfigHub only; no Targets, Workers, or live infrastructure)
- creates: 5 spaces, 5 triggers, 2 filters, 4 base units, 12 cloned units,
  3 violation units, 1 divergence revision
- idempotent: re-running skips existing entities (`exists, skipping`)
- cleanup: none by design (demo data persists); manual teardown documented in
  AI_START_HERE.md

### `./verify.sh`

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
