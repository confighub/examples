# Initiatives Demo

Scripts to populate a ConfigHub space with 5 compliance initiatives, each backed by a Kyverno CEL policy. Uses the same app units as the [promotion demo](../promotion-demo-data/) — aichat, website, docs, eshop, and portal — so both demos share a consistent data model.

Use this to explore the Initiatives feature in ConfigHub: filtering units, tracking remediation progress, setting priorities and deadlines, and (optionally) running automated policy checks via vet-kyverno.

## Prerequisites

- [`cub` CLI](https://docs.confighub.com/get-started/setup/#install-the-cli) installed and on PATH.
- Run `cub upgrade` to ensure you have the latest build.
- Authenticated to ConfigHub: `cub auth login`
- The `promotion-demo-data/` directory must be present alongside this one (it ships with this repo).

## Quick Start

```bash
./setup.sh      # Create the demo space, units, and initiatives
./cleanup.sh    # Delete everything when you're done
```

By default the scripts target `https://app.confighub.com` and create a space named `initiatives-demo`. Override with environment variables:

```bash
CONFIGHUB_URL=https://my-server.com SPACE=my-space ./setup.sh
```

## What Gets Created

### Space

One space: `initiatives-demo`

### Units (18)

Kubernetes manifests sourced from `promotion-demo-data/config-data/`, labeled with `App` and `AppOwner` to match the promotion demo model. The units have naturally varied compliance characteristics — APIs and frontends are well-configured, while databases, caches, and workers have gaps that initiatives can detect.

| App | Owner | Units |
|-----|-------|-------|
| aichat | Support | api, frontend, postgres, redis, worker |
| portal | Support | api, frontend, postgres |
| eshop | Product | api, frontend, postgres, redis, worker |
| docs | Product | server, search |
| website | Marketing | web, cms, postgres |

### Initiatives (5)

Each initiative is a **View** with a **Filter** (selects units by App or AppOwner label) and initiative metadata stored in Labels and Annotations. An optional **Trigger** runs `vet-kyverno` against matched units when a Ready worker is connected.

| Initiative | Priority | Status | Filter | Pass/Fail |
|----------|----------|--------|--------|-----------|
| Liveness and Readiness Probes | HIGH | in_progress | AppOwner = Support | 4 / 4 |
| Image Registry Restriction | HIGH | in_progress | AppOwner = Product | 4 / 3 |
| Run-As-NonRoot Enforcement | MEDIUM | in_progress | App = aichat | 2 / 3 |
| Resource Limits Enforcement | MEDIUM | draft | App = website | 2 / 1 |
| Disallow Host Ports | LOW | **completed** | App = docs | 2 / 0 |

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

## Using vet-kyverno Triggers

If a Ready bridge worker with `vet-kyverno` support is detected when `setup.sh` runs, a Trigger is created (disabled by default) for each initiative. The Trigger runs the embedded Kyverno CEL policy against every unit matched by the initiative filter.

To set up a worker:

1. Create a worker in your space:
   ```bash
   cub worker create --space initiatives-demo kyverno-worker
   ```

2. Start `cub-worker` with the worker credentials:
   ```bash
   cub-worker
   ```

3. Once the worker is Ready, re-run `./setup.sh` to attach triggers to all initiatives.

4. Enable a trigger in the ConfigHub UI (initiative settings → Trigger → Enable) and mutate a unit to fire it.

## Exploring the Data

```bash
# List all units in the demo space
cub unit list --space initiatives-demo

# Filter by app
cub unit list --space initiatives-demo --where "Labels.App = 'aichat'"

# Filter by owner
cub unit list --space initiatives-demo --where "Labels.AppOwner = 'Support'"

# List all initiative views
cub view list --space initiatives-demo --where "Labels.initiative = 'true'"

# Filter by priority
cub view list --space initiatives-demo --where "Labels.initiative-priority = 'HIGH'"

# Filter by status
cub view list --space initiatives-demo --where "Labels.initiative-status = 'in_progress'"
```
