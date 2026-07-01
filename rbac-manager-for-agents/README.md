# cub-rbac — RBAC Manager for Agents

`cub-rbac` manages **Kubernetes RBAC config as data** (Role, ClusterRole,
RoleBinding, ClusterRoleBinding, ServiceAccount) stored in ConfigHub Units
across a fleet of cluster-Spaces. It brings the capabilities of the
[RBAC Manager web app](../rbac-manager) to the terminal, in a form designed for
an **AI agent**: fleet inventory, effective-access ("who can") queries, RBAC
hygiene findings, and guardrailed edits — paired with agent Skills that teach an
agent when and how to use it.

See [PROPOSAL.md](./PROPOSAL.md) for the design rationale and roadmap.

> **Scope:** Kubernetes RBAC only. ConfigHub's own per-entity permissions are
> out of scope.

## How it works

`cub-rbac` performs all ConfigHub I/O by shelling out to the `cub` CLI, so it
uses your existing `cub` session — there is no separate login. The one thing it
keeps in Go is the RBAC **analysis engine** (parsing, Kubernetes authorization
semantics, and findings), which an agent should not recompute in-context.

It runs two ways from a single binary:

- standalone: `cub-rbac <command>`
- as a `cub` plugin: `cub rbac <command>`

## Build

```sh
make build        # -> bin/cub-rbac
```

## Quick start

```sh
cub auth login                                # if not already signed in (interactive)
bin/cub-rbac preflight                        # verify cub is installed and the session is valid
bin/cub-rbac snapshot                         # fleet RBAC inventory (JSON; add -o table)
bin/cub-rbac list --kind ClusterRoleBinding   # explorer (JSON; add -o table)
bin/cub-rbac who-can get secrets              # effective access: who can do X
bin/cub-rbac access ServiceAccount:apps/ci-deployer   # inverse: what can SUBJECT do
bin/cub-rbac findings --severity high         # RBAC hygiene issues

# Guardrailed writes — dry-run by default; --commit to apply
bin/cub-rbac edit add-verb prod/rbac --role-kind ClusterRole --role viewer --rule 0 --verb get
bin/cub-rbac fleet-edit add-verb --where "Space.Labels.Environment = 'dev'" \
  --role-kind ClusterRole --role developer --rule 0 --verb deletecollection   # bulk edit
bin/cub-rbac promote --where "Space.Labels.Environment = 'staging'"            # upgrade downstreams from upstream
bin/cub-rbac guardrails install -o table      # plan the policy pack (add --commit to apply)
bin/cub-rbac guardrails status                # Units with ApplyWarnings / ApplyGates
```

All read commands (and the fleet write commands `fleet-edit` / `promote`) scope
the fleet with a single ConfigHub Unit `--where` filter, plus opinionated
label shorthands — `--component`, `--environment`, `--region`, `--owner`,
`--layer`, `--variant` — that expand to `Space.Labels.<Key> = '<value>'`. A
single Unit filter can reference Unit, Space, and Target metadata (e.g.
`--where "Target.ProviderType = 'OCI'"` or `--where "Slug = 'rbac'"`), so the
server does all the scoping and only the matching Units' resources are fetched.

ConfigHub `where` is **flat AND-only** — no parentheses and no `OR`. The label
shorthands are ANDed onto any raw `--where` with a bare `AND`; a parenthesized
clause is rejected with `invalid attribute name`. To express a union, run the
command once per branch.

`list` and `who-can` additionally accept `--cluster` / `--namespace` as
**client-side display filters** (the cluster key is the Target slug, or the
Space slug for unbound Units); they narrow the printed rows, not the server
query.

Reads also take `-o json` (default) or `-o table`. Write commands (`edit`,
`fleet-edit`, `promote`, `guardrails install`) are **dry-run by default** and
require an explicit `--commit`; committing an `edit` also requires
`--change-desc`.

> **Tip:** so every Unit is targeted and `Target.Slug` is a consistent
> grouping key, base/template Units can bind a dummy Target with a Noop
> ProviderType — server-hosted and never applied — rather than being left
> unbound.

## Agent Skills

`skills/` holds Agent Skills (ConfigHub skills format) that teach an agent when
and how to use these commands:

- **rbac-audit** — inventory & explorer (`snapshot`, `list`). *(read)*
- **rbac-whocan** — effective access (`who-can`, `access`). *(read)*
- **rbac-findings** — RBAC hygiene (`findings`). *(read)*
- **rbac-edit** — guardrailed structured edits (`edit`). *(write)*
- **rbac-fleet** — bulk edits + variant propagation (`fleet-edit`, `promote`). *(write)*
- **rbac-guardrails** — policy pack install/status (`guardrails`). *(write)*

## Sister examples

These companion examples show how the same agentic RBAC tool can sit beside
Helm-sourced ConfigHub data:

- [RBAC Manager Over A Redis Helm Chart](../rbac-manager-over-redis/README.md)
  shows the one-chart pattern: Redis is loaded into ConfigHub, and `cub-rbac`
  inspects or proposes guarded edits to the RBAC Units produced by that chart.
- [Redis Platform With RBAC Guardrails](../redis-platform-with-rbac-guardrails/README.md)
  shows the larger-product pattern: Redis is one part of an app or platform, and
  RBAC guardrails are managed across multiple components and variants.

## Status

Implemented (M0–M4):

- Dual-mode binary, `cub` exec helper, `preflight` auth gate, `version`.
- RBAC analysis engine (`internal/rbac`): domain model + Kubernetes
  authorization semantics (wildcards, subresources, resourceNames,
  nonResourceURLs, ClusterRole aggregation, binding resolution), effective-access
  (`who-can` / `access`), six hygiene analyzers, and structured-edit compilers —
  ported from the web app with its test fixtures as Go table tests.
- Fleet snapshot loader (`internal/snapshot`): discovers Kubernetes/YAML Units
  and extracts RBAC resources server-side via `cub`, joined with Unit/Space/
  Target metadata.
- Read commands: `snapshot`, `list`, `who-can`, `access`, `findings`.
- Write commands: `edit` (add/remove verb & subject via server-side yq,
  dry-run→commit, `--change-desc`, never bypasses gates), `fleet-edit` (the same
  edits across many Units via `--where`, one server request), `promote`
  (override-preserving upstream→downstream upgrade), and `guardrails`
  install/status (Warn=true Trigger pack + shared Filter, dry-run plan→commit).
- Agent Skills (read + write) with evals.

All five proposal milestones (M0–M4) are implemented. See
[PROPOSAL.md](./PROPOSAL.md) for the design and roadmap.
