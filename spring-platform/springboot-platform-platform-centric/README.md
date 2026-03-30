# Spring Boot Platform (Experimental ADTP View)

One platform organizing multiple apps: Platform → Apps → Deployments → Targets.

**Note:** ADTP is experimental. The model is sound but tooling is incomplete.

## This View

This is the **Experimental ADTP (Platform → Apps → Deployments → Targets)** view of the Spring platform model.

Use this example to understand:

- How one platform organizes multiple apps
- What is platform-owned vs app-owned
- How apps inherit platform policies
- Platform-wide discovery commands

This is the best place to understand platform-centric operations.

## The Same Underlying Model

This is one of three views of the same underlying system:

| View | Example | Core question answered |
|------|---------|------------------------|
| Plain ConfigHub | [`springboot-platform-app`](../springboot-platform-app/) | How does `cub-gen` transform app + platform into governed operational config? |
| ADT | [`springboot-platform-app-centric`](../springboot-platform-app-centric/) | How do I understand one app across deployments and targets? |
| **Experimental ADTP** | **This example** | How do I make platform explicit above apps and deployments? |

All three examples share the same mutation routes and truth matrix structure. This example currently supports noop targets only.

## What Is Real Today

| Capability | Status |
|------------|--------|
| Generator transformation | Real |
| Field lineage / explain-field | Real |
| ConfigHub mutation storage | Real |
| Mutation history / audit trail | Real |
| Refresh preview | Real |
| Real Kubernetes delivery | Noop only (real targets not implemented in this example) |
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
| Platform-as-resource in ConfigHub | Experimental (label-based grouping only) |

## Quick Start

```bash
cd spring-platform/springboot-platform-platform-centric

# Preview the platform and all apps (read-only)
./setup.sh --explain
cat platform-map.json | jq

# Create everything (6 spaces, 5 units)
./setup.sh

# Verify
./verify.sh

# Clean up
./cleanup.sh
```

## Artifact Chain

The platform organizes apps through deployments to targets:

```
Platform: springboot-platform
│
├── Provides:
│   ├── managed-datasource (postgres-shared)
│   ├── runtime-hardening (security defaults)
│   └── observability (health, SLOs)
│
├── Controls (blocked fields):
│   ├── spring.datasource.*
│   └── securityContext.*
│
├── App: inventory-api
│   ├── Deployment: dev   → Space: inventory-api-dev
│   ├── Deployment: stage → Space: inventory-api-stage
│   └── Deployment: prod  → Space: inventory-api-prod
│
└── App: catalog-api
    ├── Deployment: dev   → Space: catalog-api-dev
    └── Deployment: prod  → Space: catalog-api-prod
```

### The Platform

**springboot-platform**: A Heroku-like Spring Boot platform providing managed services and runtime policies.

| Provides | Description |
|----------|-------------|
| Managed Datasource | PostgreSQL with HA, encryption, backups |
| Runtime Hardening | runAsNonRoot, mTLS sidecar |
| Observability | Health endpoints, SLO targets |

### The Apps

Two apps run on this platform:

| App | Description | Deployments |
|-----|-------------|-------------|
| `inventory-api` | Inventory management service | dev, stage, prod |
| `catalog-api` | Product catalog service | dev, prod |

Both apps inherit the platform's policies and get the same managed services.

### Platform Discovery

```bash
# See what the platform provides to all apps
./platform.sh --summary

# See which apps run on this platform
./platform.sh --apps

# Check field ownership for any app
./platform.sh --explain-field spring.datasource.url
```

## Mutation Routes

When you change a field in any app's operational config, one of three things happens:

| Route | When | Example field |
|-------|------|---------------|
| Apply here | App-owned, safe to mutate locally | `feature.inventory.reservationMode` |
| Lift upstream | App-owned, but needs source change | `spring.cache.*` |
| Block/escalate | Platform-owned | `spring.datasource.*` |

### Platform-Owned Fields

The platform controls fields that apps should never diverge:

```bash
./platform.sh --explain-field spring.datasource.url
# → BLOCKED: platform provides managed-datasource
```

### App-Owned Fields

Apps can mutate their own feature flags:

```bash
./platform.sh --explain-field feature.inventory.reservationMode
# → MUTABLE: app-owned, safe to change

# Mutate on inventory-api
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: reservation mode → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

# Mutate on catalog-api
cub function do --space catalog-api-prod --unit catalog-api \
  --change-desc "apply-here: enable recommendations" \
  set-env catalog-api "FEATURE_CATALOG_RECOMMENDATIONSENABLED=true"
```

## Compare This View To The Other Two

| Aspect | Plain ConfigHub | ADT | Experimental ADTP (this) |
|--------|-----------------|-----|--------------------------|
| Focus | Generator transformation | App across environments | Platform organizing apps |
| Entry question | How does config get generated? | How does my app deploy? | How do I manage multiple apps? |
| Key insight | Field lineage → mutation routes | Deployments → spaces | Platform → apps |
| Best for | Understanding the machinery | Operating one app | Platform team operations |

## Key Files

### Public Commands

| Command | Purpose |
|---------|---------|
| `./setup.sh --explain` | Preview the platform view |
| `./setup.sh` | Create all spaces and units |
| `./verify.sh` | Verify setup |
| `./cleanup.sh` | Delete all created objects |
| `./platform.sh --summary` | Platform capabilities |
| `./platform.sh --apps` | Apps on this platform |
| `./platform.sh --explain-field` | Field ownership |

### Configuration

| Path | Purpose |
|------|---------|
| `platform-map.json` | Machine-readable ADTP map |
| `platform.yaml` | Platform definition |
| `apps/catalog-api/` | App definition (catalog-api) |
| `../shared/confighub/` | Shared unit YAMLs (inventory-api) |

## Prerequisites

**Basic:**
- `cub` CLI, authenticated (`cub auth login`)
- `jq`

**Real Kubernetes deployment:**
- `kind`, `kubectl`, Docker
- See [`../springboot-platform-app/README.md`](../springboot-platform-app/README.md) for setup

## AI Handoff

- AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
