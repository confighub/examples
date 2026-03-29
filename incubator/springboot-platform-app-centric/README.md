# Spring Boot Platform App (App-Centric View)

One app. Three deployments. Three mutation outcomes.

## The App

**inventory-api**: A Spring Boot service that manages inventory items with feature flags and runtime tuning.

## The Deployments

| Deployment | Space | Purpose |
|------------|-------|---------|
| dev | `inventory-api-dev` | Development iteration |
| stage | `inventory-api-stage` | Validation before prod |
| prod | `inventory-api-prod` | Production workload |

Each deployment is a ConfigHub space containing one unit (`inventory-api`).

## Target Modes

Targets control where a unit delivers. This example supports three modes:

| Mode | What it does | Cluster required? |
|------|--------------|-------------------|
| **noop** (default) | Apply workflow works. Noop worker accepts but doesn't deploy. | No |
| unbound | No targets. Units exist in ConfigHub only. | No |
| real | Apply delivers to live Kubernetes cluster. | Yes |

The default (`./setup.sh`) uses noop targets so you can see the full mutation-to-apply workflow immediately without a cluster.

**Alternative delivery modes:** This example uses direct Kubernetes delivery. For Flux OCI or Argo OCI delivery (where Flux/ArgoCD reconciles from an OCI artifact), see [`global-app-layer/single-component`](../global-app-layer/single-component/).

## The Three Mutation Outcomes

When you change a field in this app's operational config, one of three things happens:

| Outcome | When | Example |
|---------|------|---------|
| **Apply here** | Field is app-owned and safe to mutate locally | `feature.inventory.reservationMode` |
| **Lift upstream** | Change should flow back to app source | `spring.cache.*` (adding Redis) |
| **Block / escalate** | Field is platform-owned and requires approval | `spring.datasource.*` |

See the [flows/](./flows/) directory for detailed walkthroughs of each outcome.

## Quick Start

```bash
# See what this will do (read-only)
./setup.sh --explain
./setup.sh --explain-json | jq

# Set up with noop targets (default, no cluster needed)
./setup.sh

# Verify everything is consistent
./verify.sh

# Clean up when done
./cleanup.sh
```

## Setup Options

| Command | What it does |
|---------|--------------|
| `./setup.sh --explain` | Show the ADT view (read-only) |
| `./setup.sh --explain-json` | Machine-readable setup plan |
| `./setup.sh` | Create spaces, units, noop targets, and apply |
| `./setup.sh --confighub-only` | Create spaces and units without targets |
| `./setup.sh --with-targets` | Deploy to real Kubernetes (requires cluster + worker) |

## AI Handoff

- AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Copyable prompts: [`prompts.md`](./prompts.md)
- Stable contracts: [`contracts.md`](./contracts.md)
- Machine-readable app/deployment map: [`deployment-map.json`](./deployment-map.json)

## How This Relates to springboot-platform-app

This is an app-centric wrapper, not a fork. All implementation lives in [`../springboot-platform-app/`](../springboot-platform-app/).

| Here | There |
|------|-------|
| App/deployment/target story | Space/unit/worker implementation |
| `./setup.sh` delegates | `./confighub-setup.sh` does the work |
| `./verify.sh` delegates | `./verify.sh` checks fixtures |
| `./cleanup.sh` delegates | `./confighub-cleanup.sh` deletes |
| Three flow docs | Three change docs + lift-upstream bundle + block-escalate boundary |
| Field ownership understanding | `generator/` shows the transformation |

Use this example when you want to understand the story. Use `springboot-platform-app` when you need the full implementation detail.

## Understanding Field Ownership

To understand why some fields are "apply-here" vs "block/escalate", see the generator that transforms app inputs into operational config:

```bash
# See how inputs become outputs
../springboot-platform-app/generator/render.sh --explain

# Field-by-field mapping
../springboot-platform-app/generator/render.sh --trace
```

The generator shows how platform policy (`upstream/platform/runtime-policy.yaml`) gets injected into the Kubernetes Deployment, making those fields platform-owned and blocked from local mutation.

## Try a Mutation

After setup, try changing a feature flag:

```bash
# See current value
cub unit get --space inventory-api-prod inventory-api --json | jq '.Unit.Objects'

# Change the reservation mode (apply-here outcome)
cub function do --space inventory-api-prod --unit inventory-api \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic

# Re-apply to target
cub unit apply --space inventory-api-prod inventory-api
```

This mutation applies directly because `feature.inventory.*` fields route to `apply-here`.

## Files

```
springboot-platform-app-centric/
  README.md                 # This file
  AI_START_HERE.md          # AI assistant guide
  prompts.md                # Copyable AI prompts
  contracts.md              # Stable command contracts
  deployment-map.json       # Machine-readable ADT map
  setup.sh                  # Setup (delegates)
  demo.sh                   # Demo the three outcomes
  verify.sh                 # Verify (delegates)
  cleanup.sh                # Cleanup (delegates)
  flows/
    apply-here.md           # Apply-here walkthrough
    lift-upstream.md        # Lift-upstream walkthrough
    block-escalate.md       # Block/escalate walkthrough
```

## Prerequisites

- `cub` CLI installed and authenticated (`cub auth login`)
- `jq` for JSON handling

For real Kubernetes deployment (`--with-targets`):
- Kind or other Kubernetes cluster
- Docker for building images
- See `../springboot-platform-app/README.md` for full prerequisites

## Cleanup

```bash
./cleanup.sh
```

This deletes all ConfigHub spaces labeled with this example.
