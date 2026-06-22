# Contracts: promoter

Stable, machine-checkable behavior for this example. See
[EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for the format.
promoter is a webapp, not a CLI; these contracts cover the ConfigHub artifacts
it reads and writes, which automation can assert against.

### Storage Space

- slug: `promoter`
- created-or-updated by the app on first use (idempotent; `allow_exists`)
- labels: `app=promoter`
- mutates live infra: no

### Workflow unit

- one Unit per workflow in the `promoter` Space
- `ToolchainType`: `AppConfig/YAML`
- labels: `app=promoter`
- `Data`: the workflow document (YAML), schema below
- discoverable via: `cub unit list --space promoter --where "Labels.app = 'promoter'"`

### Workflow document schema

- `apiVersion`: `promoter.confighub.com/v1`
- `name`: string (display name)
- `statusLabel`: Space-label key to read each variant's live status from
  (default `Status`)
- `stages`: ordered list of `{ name, components: [{ component, variant }] }`
- `component` / `variant` correspond to the `Component` / `Variant` Space labels
- the document does **not** store status

### Status (read-only, from Space labels)

- a variant's status is the value of its Space's `statusLabel` label
- the app reads it and **never writes it**; it is set by an operator/agent (or
  the CLI: `cub space update --patch <space> --label "Status=Ready"`)
- value mapping (case-insensitive): `Ready|Healthy|Synced|Deployed|Succeeded`
  → ready; `Progressing|Deploying|Running|Pending` → in progress;
  `Failed|Degraded|Error|Unhealthy` → failed; absent → no status
- the pipeline view polls Spaces every 5s; a stage's status is the rollup of its
  components; the Promote gate for a stage opens only when its upstream stage
  is ready

### Component/variant catalog (read-only)

- a component is a Space `Component` label value
- a variant is a Space with that `Component` label; its name is the `Variant`
  label (falling back to the Space slug if absent)
- the app reads Spaces only; it never writes component/variant Space metadata

### Promote action

- mechanism: `patchUnit` with `upgrade=true` against each unit in the target
  variant-Space (equivalent to `cub unit update --patch --upgrade`)
- precondition: every target unit's `UpstreamSpaceID` equals the previous
  stage's chosen variant Space — otherwise the action is disabled and reports
  the reason; it never falls back to copying `Data`
- gate: a stage's Promote opens only when its upstream stage is `succeeded`
  (ready) per the live Space-label status
- optional: applies the upgraded units (`cub unit apply`) when "Apply after
  upgrade" is checked
- the app does not record status; the resulting live status is reported back by
  ConfigHub/agents via the variant Space's label
