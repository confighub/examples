# Spring Boot Platform App (ADT View)

One app, three environments, three kinds of config change. No cluster required.

```bash
./setup.sh --explain
```

This example shows `inventory-api` deployed across dev, stage, and prod. In
current ConfigHub language, `inventory-api` is the Component and dev/stage/prod
are Deployment Variants. Each Deployment Variant is represented as a ConfigHub
space with its own unit and target. You can mutate fields, apply to targets,
and see what routes where.

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
| (default) | Noop targets — apply path can be exercised, no cluster needed |
| `--confighub-only` | Spaces and units only |
| `--with-targets` | Real Kubernetes delivery (requires cluster) |

## What You'll Work With

```
Component: inventory-api
├── Deployment Variant: dev   → inventory-api-dev   → noop target
├── Deployment Variant: stage → inventory-api-stage → noop target
└── Deployment Variant: prod  → inventory-api-prod  → noop target
```

Each space contains one unit (`inventory-api`) that can be mutated and applied independently.

## Mutation Routes

```bash
./demo.sh   # see all three routes in action
```

| Route | Field example | What happens here | Current product path |
|-------|---------------|-------------------|----------------------|
| Apply here | `feature.inventory.*` | Teaching shortcut: mutate with ConfigHub `set-env`, then apply to target | `cub-gen springboot set-embedded-config` mutates embedded `application.yaml` |
| Lift upstream | `spring.cache.*` | Bundle produced, needs source change | `cub-gen springboot validate-mutation` routes it back to source; automated PR creation is not implemented |
| Block/escalate | `spring.datasource.*` | Boundary is documented, not server-enforced here | `cub-gen springboot validate-mutation` returns `BLOCKED`; backend enforcement is still future work |

### Try a Mutation

This is the teaching-era ConfigHub mutation path. Use
`cub-gen springboot set-embedded-config` in the maintained `cub-gen`
Spring example when you want the productized embedded-config apply-here path.

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
See [`../BRING-YOUR-OWN-APP.md`](../BRING-YOUR-OWN-APP.md) if you want to adapt the example to your own service.

AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
