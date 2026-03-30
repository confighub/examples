# Spring Boot Platform App (ADT View)

One app, three environments, three kinds of config change. No cluster required.

```bash
./setup.sh --explain
```

This example shows `inventory-api` deployed across dev, stage, and prod. Each deployment is a ConfigHub space with its own unit and target. You can mutate fields, apply to targets, and see what routes where.

## Quick Start

```bash
./setup.sh --explain    # see the ADT model (read-only)
./setup.sh              # create spaces, units, noop targets
./verify.sh             # check consistency
./cleanup.sh            # delete everything
```

### Setup Options

| Flag | Effect |
|------|--------|
| (default) | Noop targets — apply works, no cluster needed |
| `--confighub-only` | Spaces and units only |
| `--with-targets` | Real Kubernetes delivery (requires cluster) |

## What You'll Work With

```
App: inventory-api
├── dev   → inventory-api-dev   → noop target
├── stage → inventory-api-stage → noop target
└── prod  → inventory-api-prod  → noop target
```

Each space contains one unit (`inventory-api`) that can be mutated and applied independently.

## Mutation Routes

```bash
./demo.sh   # see all three routes in action
```

| Route | Field example | What happens |
|-------|---------------|--------------|
| Apply here | `feature.inventory.*` | Mutate in ConfigHub, apply to target |
| Lift upstream | `spring.cache.*` | Bundle produced, needs source change |
| Block/escalate | `spring.datasource.*` | Platform-owned, blocked |

### Try a Mutation

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

cub unit apply --space inventory-api-prod inventory-api
```

### See Field Ownership

```bash
cat ../shared/field-routes.yaml
```

## Key Files

| Path | Purpose |
|------|---------|
| `deployment-map.json` | Machine-readable ADT structure |
| `flows/*.md` | Detailed mutation walkthroughs |

## Prerequisites

- `cub` CLI, authenticated
- `jq`
- Kind/kubectl/Docker for real deployment

## Related

See [`../README.md`](../README.md) for how the three examples compare.

AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
