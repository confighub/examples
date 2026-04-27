# Initiatives Demo

Scripts to populate a ConfigHub space with 5 compliance initiatives, each backed by a Kyverno CEL policy. Uses the same app units as the [promotion demo](../promotion-demo-data/) — aichat, website, docs, eshop, and portal — so both demos share a consistent data model.

Use this to explore the Initiatives feature in ConfigHub: filtering units, tracking remediation progress, setting priorities and deadlines, and (optionally) running automated policy checks via vet-kyverno.

## Prerequisites

- [`cub` CLI](https://docs.confighub.com/get-started/setup/#install-the-cli) installed and on PATH.
- Run `cub upgrade` to ensure you have the latest build.
- Authenticated to ConfigHub: `cub auth login`
- `kind`, `kubectl`, `docker`, `jq` on PATH — `setup.sh` stands up a local kind cluster and installs a vet-kyverno worker into it so initiative triggers have somewhere to run.
- The `promotion-demo-data/` and `custom-workers/kyverno/` directories must be present alongside this one (they ship with this repo).

## Quick Start

```bash
./setup.sh      # Create kind cluster, workers, demo space, units, and initiatives
./cleanup.sh    # Delete everything (space + kind cluster) when you're done
```

By default the scripts target the server from your current `cub` context, create a space named `initiatives-demo`, and a kind cluster of the same name. Override with environment variables:

```bash
CONFIGHUB_URL=https://my-server.com SPACE=my-space CLUSTER_NAME=my-kind ./setup.sh
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

`setup.sh` installs a `vet-kyverno` worker into the `initiatives-demo` space (running in the local kind cluster), so a Trigger is created for each initiative. The Trigger runs the embedded Kyverno CEL policy against every unit matched by the initiative filter.

The worker search is intentionally scoped to this demo's own space — no other demo's worker can be silently picked up, and tearing down this demo never touches another demo's state.

To exercise a trigger, mutate one of the units in the ConfigHub UI (or via `cub unit update`) — failures land in `ApplyWarnings` (advisory) for in-progress initiatives, or `ApplyGates` (blocking) for the completed "Disallow Host Ports" initiative.

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

## Using with AI (the AI ↔ GUI bridge)

This demo works with the `confighub-ai-demo` repo's skills and slash commands. The AI and the GUI operate on the same initiative Views — an initiative created by the AI appears on the GUI kanban, and an initiative created in the GUI can be picked up by the AI.

**Full guide:** [`AI_GUI_BRIDGE_GUIDE.md`](./AI_GUI_BRIDGE_GUIDE.md) — Direction 1 (AI → GUI) and Direction 2 (GUI → AI) explained step by step with example commands and output.

**Quick start:**

```bash
# 1. Set up the demo
./setup.sh

# 2. Open Claude in the confighub-ai-demo repo
cd ../confighub-ai-demo && claude

# 3. Try it
"Show me the initiatives in the initiatives-demo space"
"Run the image registry restriction initiative"
"Create a new initiative to pin all :latest image tags"
```
