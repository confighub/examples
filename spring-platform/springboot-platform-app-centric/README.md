# Spring Boot Platform App (ADT View)

One app across deployments and targets: App → Deployments → Targets.

## This View

This is the **ADT (App → Deployments → Targets)** view of the Spring platform model.

Use this example to understand:

- How one app (`inventory-api`) maps to multiple deployments
- How each deployment becomes a ConfigHub space
- How targets control where config delivers
- The three mutation outcomes for any field change

This is the best place to understand the app-deployment-target model.

## The Same Underlying Model

This is one of three views of the same underlying system:

| View | Example | Core question answered |
|------|---------|------------------------|
| Plain ConfigHub | [`springboot-platform-app`](../springboot-platform-app/) | How does `cub-gen` transform app + platform into governed operational config? |
| **ADT** | **This example** | How do I understand one app across deployments and targets? |
| Experimental ADTP | [`springboot-platform-platform-centric`](../springboot-platform-platform-centric/) | How do I make platform explicit above apps and deployments? |

All three examples share the same mutation routes and truth matrix structure. This example supports the same target modes as the core example.

## What Is Real Today

| Capability | Status |
|------------|--------|
| Generator transformation | Real |
| Field lineage / explain-field | Real |
| ConfigHub mutation storage | Real |
| Mutation history / audit trail | Real |
| Refresh preview | Real |
| Real Kubernetes delivery | Real (Kind cluster) |
| Noop target simulation | Real |
| Running app HTTP verification | Real |

## What Is Simulated Today

| Capability | Status |
|------------|--------|
| Noop target mode | Simulated (accepts apply, does not deliver) |

## What Is Not Implemented Yet

| Capability | Status |
|------------|--------|
| `lift upstream` automated PR | Bundle exists, no automated PR creation |
| `block/escalate` server-side enforcement | Documented, not enforced |
| Flux/Argo delivery path | See `global-app-layer` examples |

## Quick Start

```bash
cd spring-platform/springboot-platform-app-centric

# Preview the ADT view (read-only)
./setup.sh --explain
./setup.sh --explain-json | jq

# Create spaces, units, noop targets, and apply (default)
./setup.sh

# Verify
./verify.sh

# Clean up
./cleanup.sh
```

### Setup Options

| Command | What it does |
|---------|--------------|
| `./setup.sh` | Full setup with noop targets (default) |
| `./setup.sh --confighub-only` | Spaces and units only, no targets |
| `./setup.sh --with-targets` | Deploy to real Kubernetes (requires cluster) |

## Artifact Chain

The app flows through deployments to targets:

```
App: inventory-api
│
├── Deployment: dev   → Space: inventory-api-dev   → Target: noop/real
├── Deployment: stage → Space: inventory-api-stage → Target: noop/real
└── Deployment: prod  → Space: inventory-api-prod  → Target: noop/real
```

### The App

**inventory-api**: A Spring Boot service managing inventory items with feature flags and runtime tuning.

### The Deployments

| Deployment | Space | Purpose |
|------------|-------|---------|
| dev | `inventory-api-dev` | Development iteration |
| stage | `inventory-api-stage` | Validation before prod |
| prod | `inventory-api-prod` | Production workload |

Each deployment is a ConfigHub space containing one unit (`inventory-api`).

### The Targets

| Mode | What it does | Cluster required? |
|------|--------------|-------------------|
| noop (default) | Apply workflow works, no delivery | No |
| unbound | No targets, units exist only | No |
| real | Apply delivers to Kubernetes | Yes |

## Mutation Routes

When you change a field in this app's operational config, one of three things happens:

| Route | When | Example field |
|-------|------|---------------|
| Apply here | App-owned, safe to mutate locally | `feature.inventory.reservationMode` |
| Lift upstream | App-owned, but needs source change | `spring.cache.*` |
| Block/escalate | Platform-owned | `spring.datasource.*` |

### Apply Here

```bash
# Change the reservation mode for prod
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: reservation mode strict → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

# Re-apply to target
cub unit apply --space inventory-api-prod inventory-api
```

This mutation is stored in ConfigHub and survives future refreshes.

### Lift Upstream

See: [`flows/lift-upstream.md`](./flows/lift-upstream.md)

The Redis caching request would change upstream inputs, not just ConfigHub state.

### Block/Escalate

See: [`flows/block-escalate.md`](./flows/block-escalate.md)

The datasource field is platform-owned and should be blocked or escalated.

### Understanding Field Ownership

To see why a field routes to a particular outcome, check the field routing rules in `../shared/field-routes.yaml` or use the core example's generator:

```bash
# View field routing rules
cat ../shared/field-routes.yaml

# Or use the core example's explain-field command
../springboot-platform-app/generator/render.sh --explain-field spring.datasource.url
# → BLOCKED: generator injects from platform policy
```

## Compare This View To The Other Two

| Aspect | Plain ConfigHub | ADT (this) | Experimental ADTP |
|--------|-----------------|------------|-------------------|
| Focus | Generator transformation | App across environments | Platform organizing apps |
| Entry question | How does config get generated? | How does my app deploy? | How do I manage multiple apps? |
| Key insight | Field lineage → mutation routes | Deployments → spaces | Platform → apps |
| Best for | Understanding the machinery | Operating one app | Platform team operations |

## Key Files

### Public Commands

| Command | Purpose |
|---------|---------|
| `./setup.sh --explain` | Preview the ADT view |
| `./setup.sh` | Create spaces, units, targets, apply |
| `./verify.sh` | Verify setup |
| `./cleanup.sh` | Delete all created objects |
| `./demo.sh` | Show the three mutation outcomes |

### Flow Documentation

| Path | Purpose |
|------|---------|
| `flows/apply-here.md` | Apply-here mutation walkthrough |
| `flows/lift-upstream.md` | Lift-upstream mutation walkthrough |
| `flows/block-escalate.md` | Block/escalate mutation walkthrough |

### Configuration

| Path | Purpose |
|------|---------|
| `deployment-map.json` | Machine-readable ADT map |

## Prerequisites

**Basic:**
- `cub` CLI, authenticated (`cub auth login`)
- `jq`

**Real Kubernetes deployment:**
- `kind`, `kubectl`, Docker
- See [`../springboot-platform-app/README.md`](../springboot-platform-app/README.md) for setup

## AI Handoff

- AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Copyable prompts: [`prompts.md`](./prompts.md)
- Stable contracts: [`contracts.md`](./contracts.md)
