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
- `stages`: ordered list of `{ name, components: [{ component, variant }] }`
- `status`: map keyed by `"<stage>/<component>"` →
  `{ state, promotedRevision?, at?, by? }`, where `state` ∈
  `pending | succeeded | failed | unknown`
- `component` / `variant` correspond to the `Component` / `Variant` Space labels

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
- optional: applies the upgraded units (`cub unit apply`) when "Apply after
  upgrade" is checked
- records: on success, sets the (stage, component) status to `succeeded` with
  the new head revision in the workflow document
