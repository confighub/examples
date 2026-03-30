# Spring Boot Platform (Experimental ADTP View)

One platform, two apps, five deployments. See what's platform-owned vs app-owned.

**Experimental:** The model is sound but tooling is incomplete. Noop targets only.

```bash
./setup.sh --explain
```

This example shows `springboot-platform` providing managed services to `inventory-api` (3 envs) and `catalog-api` (2 envs). The platform controls datasource and security fields; apps control their own feature flags.

## Quick Start

```bash
./setup.sh --explain    # see the platform model (read-only)
./setup.sh              # create 6 spaces, 5 units, 5 noop targets
./verify.sh             # check consistency
./cleanup.sh            # delete everything
```

## What You'll Work With

```
Platform: springboot-platform
├── Provides: managed-datasource, runtime-hardening, observability
├── Controls: spring.datasource.*, securityContext.*
│
├── inventory-api
│   ├── dev → inventory-api-dev
│   ├── stage → inventory-api-stage
│   └── prod → inventory-api-prod
│
└── catalog-api
    ├── dev → catalog-api-dev
    └── prod → catalog-api-prod
```

## Platform Discovery

```bash
./platform.sh --summary                       # what the platform provides
./platform.sh --apps                          # apps on this platform
./platform.sh --explain-field spring.datasource.url  # field ownership
```

## Mutation Routes

| Route | Field example | Owner |
|-------|---------------|-------|
| Apply here | `feature.inventory.*` | App team |
| Lift upstream | `spring.cache.*` | App team |
| Block/escalate | `spring.datasource.*` | Platform team |

### Try a Mutation

```bash
# On inventory-api
cub function do --space inventory-api-prod --unit inventory-api \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

# On catalog-api
cub function do --space catalog-api-prod --unit catalog-api \
  set-env catalog-api "FEATURE_CATALOG_RECOMMENDATIONSENABLED=true"
```

## Key Files

| Path | Purpose |
|------|---------|
| `platform-map.json` | Machine-readable ADTP structure |
| `platform.yaml` | Platform definition |
| `apps/catalog-api/` | Local catalog-api config |

## Prerequisites

- `cub` CLI, authenticated
- `jq`

## Related

See [`../README.md`](../README.md) for how the three examples compare.

AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
