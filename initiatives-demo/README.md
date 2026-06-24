# Initiatives Demo

Scripts to populate ConfigHub with 5 compliance initiatives, each backed by a Kyverno CEL policy. Uses the same app units as the [promotion demo](../promotion-demo-data/) — aichat, website, docs, eshop, and portal — laid out using ConfigHub's **component model**: one Space per application component, with the well-known component labels on each Space.

Use this to explore the Initiatives feature in ConfigHub: filtering units across spaces, tracking remediation progress, setting priorities and deadlines, and (optionally) running automated policy checks via vet-kyverno.

## Prerequisites

- [`cub` CLI](https://docs.confighub.com/get-started/setup/#install-the-cli) installed and on PATH.
- Run `cub upgrade` to ensure you have the latest build.
- Authenticated to ConfigHub: `cub auth login`
- `kind`, `kubectl`, `docker`, `jq` on PATH — `setup.sh` stands up a local kind cluster and installs a vet-kyverno worker into it so initiative triggers have somewhere to run.
- The `promotion-demo-data/` and `custom-workers/kyverno/` directories must be present alongside this one (they ship with this repo).

## Quick Start

```bash
./setup.sh      # Create kind cluster, workers, component spaces, units, and initiatives
./cleanup.sh    # Delete everything (all spaces + kind cluster) when you're done
```

The scripts run against whatever ConfigHub your `cub` CLI is signed in to — the default is `hub.confighub.com`. To use a different server (e.g. ConfigHub Enterprise), switch first with `cub auth login` / `cub context use`; the scripts follow your active context.

By default they create a platform Space named `initiatives-demo`, one Space per component, and a kind cluster named after the platform Space. Override those with environment variables:

```bash
PLATFORM_SPACE=my-platform PURPOSE=my-demo CLUSTER_NAME=my-kind ./setup.sh
```

## What Gets Created

### Spaces

This demo uses the **component model**: every application component gets its own Space, and the well-known component labels live on the **Space** (not on the units).

| Space | Component | Variant | Owner | Team | Layer | Purpose |
|-------|-----------|---------|-------|------|-------|---------|
| `aichat` | aichat | base | Support | AI | App | initiatives-demo |
| `portal` | portal | base | Support | Portal | App | initiatives-demo |
| `eshop` | eshop | base | Product | Commerce | App | initiatives-demo |
| `docs` | docs | base | Product | DevEx | App | initiatives-demo |
| `website` | website | base | Marketing | Web | App | initiatives-demo |
| `initiatives-demo` | — | — | — | — | Platform | initiatives-demo |

`initiatives-demo` is the **platform Space**: it holds the shared entities — the kyverno/k8s Workers, the initiative Views and their Filters, and the vet-kyverno Triggers — but **no application units**. The `Purpose=initiatives-demo` label ties all the demo's spaces together so the initiative filters (and `cleanup.sh`) can find them regardless of how they're named.

### Units (18)

Kubernetes manifests sourced from `promotion-demo-data/config-data/`, created one per resource in their component's Space. The units have naturally varied compliance characteristics — APIs and frontends are well-configured, while databases, caches, and workers have gaps that initiatives can detect.

| Space (Component) | Owner | Units |
|-------------------|-------|-------|
| aichat | Support | aichat-api, aichat-frontend, aichat-postgres, aichat-redis, aichat-worker |
| portal | Support | portal-api, portal-frontend, portal-postgres |
| eshop | Product | eshop-api, eshop-frontend, eshop-postgres, eshop-redis, eshop-worker |
| docs | Product | docs-server, docs-search |
| website | Marketing | website-web, website-cms, website-postgres |

### Initiatives (5)

Each initiative is a **View** in the platform Space with a **Filter** and initiative metadata stored in Labels and Annotations. The Filter selects units **across the component Spaces** by their Space labels (`Space.Labels.Component` / `Space.Labels.Owner`, scoped by `Space.Labels.Purpose`). An optional **Trigger** runs `vet-kyverno` against the matched units when a Ready worker is connected.

| Initiative | Priority | Status | Filter (cross-space) | Pass/Fail |
|----------|----------|--------|----------------------|-----------|
| Liveness and Readiness Probes | HIGH | in_progress | `Space.Labels.Owner = Support` | 4 / 4 |
| Image Registry Restriction | HIGH | in_progress | `Space.Labels.Owner = Product` | 4 / 3 |
| Run-As-NonRoot Enforcement | MEDIUM | in_progress | `Space.Labels.Component = aichat` | 2 / 3 |
| Resource Limits Enforcement | MEDIUM | draft | `Space.Labels.Component = website` | 2 / 1 |
| Disallow Host Ports | LOW | **completed** | `Space.Labels.Component = docs` | 2 / 0 |

## Initiative Metadata

Initiatives use ConfigHub Labels and Annotations on Views to store their state:

### Labels

| Label | Values |
|-------|--------|
| `initiative` | `"true"` — marks the View as an initiative |
| `initiative-priority` | `HIGH`, `MEDIUM`, `LOW` |
| `initiative-status` | `draft`, `in_progress`, `completed` |

### Annotations

| Annotation | Description |
|------------|-------------|
| `initiative-description` | Human-readable goal |
| `initiative-deadline` | Target completion date (YYYY-MM-DD) |
| `initiative-completed-at` | ISO 8601 timestamp when completed |
| `initiative-trigger-id` | ID of the associated vet-kyverno Trigger |
| `initiative-check-summary` | JSON: `{passing, failing, total, checkedAt}` |

## How the Triggers Fire Across Spaces

`setup.sh` installs a `vet-kyverno` worker into the platform Space (running in the local kind cluster) and creates one Trigger per initiative there, each labeled `initiative-check=true`. A `From=Trigger` Filter (`initiative-checks`) selects those triggers, and every component Space references it via `--trigger-filter`. That's how a Trigger defined in `initiatives-demo` runs against units that live in `aichat`, `eshop`, and the rest.

The worker search is intentionally scoped to the platform Space — no other demo's worker can be silently picked up, and tearing down this demo never touches another demo's state.

To exercise a trigger, mutate one of the units (in the ConfigHub UI or via `cub unit update`) — failures land in `ApplyWarnings` (advisory) for in-progress initiatives, or `ApplyGates` (blocking) for the completed "Disallow Host Ports" initiative.

## Exploring the Data

```bash
# List the demo's spaces
cub space list --where "Labels.Purpose = 'initiatives-demo'"

# List units in one component space
cub unit list --space eshop

# List units across all component spaces by Space label
cub unit list --space "*" --where "Space.Labels.Component = 'aichat'"
cub unit list --space "*" --where "Space.Labels.Owner = 'Support'"

# List all initiative views (they live in the platform space)
cub view list --space initiatives-demo --where "Labels.initiative = 'true'"

# Filter initiatives by priority / status
cub view list --space initiatives-demo --where "Labels.initiative-priority = 'HIGH'"
cub view list --space initiatives-demo --where "Labels.initiative-status = 'in_progress'"

# Inspect a unit's gates/warnings and the failure messages behind them
cub unit get aichat-redis --space aichat -o "jq=.Unit.ApplyWarnings"
cub unit get aichat-redis --space aichat -o "jq=.Unit.ValidationResults"
```
