# Spring Boot Platform (Platform-Centric View)

One platform. Multiple apps. Shared policies.

## The Sequence

This is the **third** in a sequence of three related examples:

| # | Example | Focus | Model |
|---|---------|-------|-------|
| 1 | [`springboot-platform-app`](../springboot-platform-app/) | Generator story | How cub-gen transforms app+platform → operational |
| 2 | [`springboot-platform-app-centric`](../springboot-platform-app-centric/) | App-centric view | App → Deployments → Targets |
| 3 | **This example** | Platform-centric view | Platform → Apps → Deployments → Targets |

**Start with #1** if you want to understand the generator/authority story.
**Start with #2** if you want to understand one app across environments.
**Start with #3 (here)** if you want to understand how multiple apps share a platform.

## The Platform

**springboot-platform**: A Heroku-like Spring Boot platform providing managed services and runtime policies.

| Provides | Description |
|----------|-------------|
| Managed Datasource | PostgreSQL with HA, encryption, backups |
| Runtime Hardening | runAsNonRoot, mTLS sidecar |
| Observability | Health endpoints, SLO targets |

## The Apps

Two apps run on this platform:

| App | Description | Deployments |
|-----|-------------|-------------|
| `inventory-api` | Inventory management service | dev, stage, prod |
| `catalog-api` | Product catalog service | dev, prod |

Both apps inherit the platform's policies and get the same managed services.

## The Model

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

## Quick Start

```bash
# See the platform and all apps
./setup.sh --explain
cat platform-map.json | jq

# Create everything (5 spaces, 5 units)
./setup.sh

# Verify
./verify.sh

# Clean up
./cleanup.sh
```

## What This Adds Over app-centric

The app-centric example (#2) shows one app with three deployments. This example adds:

| Concept | app-centric | platform-centric |
|---------|-------------|------------------|
| Apps | 1 (inventory-api) | 2+ (inventory-api, catalog-api) |
| Platform | Implicit in generator | Explicit resource |
| Shared policies | Per-app field-routes | Platform-wide policies |
| Discovery | Per-app commands | Platform-wide commands |

## Platform Discovery

```bash
# See what the platform provides to all apps
./platform.sh --summary

# See which apps run on this platform
./platform.sh --apps

# Check field ownership for any app
./platform.sh --explain-field spring.datasource.url
```

## AI Handoff

- Folder-level AI entry point: [`../AI_START_HERE.md`](../AI_START_HERE.md)
- AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Canonical pacing standard: [`../../incubator/docs/ai-first-demo-standard.md`](../../incubator/docs/ai-first-demo-standard.md)
- Longer pacing guide: [`../../incubator/standard-ai-demo-pacing.md`](../../incubator/standard-ai-demo-pacing.md)

## Files

```
springboot-platform-platform-centric/
  README.md                 # This file
  AI_START_HERE.md          # AI assistant guide
  platform-map.json         # Machine-readable PADT map
  platform.yaml             # Platform definition
  setup.sh                  # Setup (creates all spaces/units)
  platform.sh               # Platform discovery commands
  verify.sh                 # Verify
  cleanup.sh                # Cleanup
  apps/
    inventory-api/          # App definition (delegates to springboot-platform-app)
    catalog-api/            # Second app definition
```

## How This Relates to the Other Examples

This example **delegates** to `springboot-platform-app` for implementation:

| Here | There |
|------|-------|
| Platform definition | Generator + policies |
| App: inventory-api | Full implementation |
| App: catalog-api | Minimal addition |
| Platform discovery | Generator discovery |

The platform-centric view **organizes** multiple apps under one platform without duplicating implementation.

## Prerequisites

- `cub` CLI installed and authenticated (`cub auth login`)
- `jq` for JSON handling

## Cleanup

```bash
./cleanup.sh
```

This deletes all ConfigHub spaces labeled with this example.
