# Spring Boot Platform App (ADT View)

One app, three environments, three kinds of config change. No cluster required.

```bash
./setup.sh --explain
```

This example shows `inventory-api` deployed across dev, stage, and prod. In
current ConfigHub language, `inventory-api` is the Component and dev/stage/prod
are Deployment Variants. Each Deployment Variant is represented as a ConfigHub
space with its own unit. You can mutate fields and see what routes where.
Delivery to a cluster is out of scope for this example; ConfigHub delivers
Releases to clusters through Argo CD or Flux.

## Quick Start

```bash
./setup.sh --explain    # see the ADT model (read-only)
./setup.sh              # create spaces and units
./verify.sh             # check consistency
./cleanup.sh            # delete everything
```

## What You'll Work With

```
Component: inventory-api
├── Deployment Variant: dev   → inventory-api-dev
├── Deployment Variant: stage → inventory-api-stage
└── Deployment Variant: prod  → inventory-api-prod
```

Each space contains one unit (`inventory-api`) that can be mutated independently.

## Mutation Routes

```bash
./demo.sh   # see all three routes in action
```

| Route | Field example | What happens here | Current product path |
|-------|---------------|-------------------|----------------------|
| Apply here | `feature.inventory.*` | Teaching shortcut: mutate with ConfigHub `set-env` | `cub-gen springboot set-embedded-config` mutates embedded `application.yaml` |
| Lift upstream | `spring.cache.*` | Bundle produced, needs source change | `cub-gen springboot validate-mutation` routes it back to source; automated PR creation is not implemented |
| Block/escalate | `spring.datasource.*` | Boundary is documented, not server-enforced here | `cub-gen springboot validate-mutation` returns `BLOCKED`; backend enforcement is still future work |

### Try a Mutation

This is the teaching-era ConfigHub mutation path. Use
`cub-gen springboot set-embedded-config` in the maintained `cub-gen`
Spring example when you want the productized embedded-config apply-here path.

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
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

## Related

See [`../README.md`](../README.md) for how the three examples compare.
See [`../BRING-YOUR-OWN-APP.md`](../BRING-YOUR-OWN-APP.md) if you want to adapt the example to your own service.

AI guide: [`AI_START_HERE.md`](./AI_START_HERE.md)
