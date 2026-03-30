# AI Start Here: Spring Boot Platform App (ADT View)

Read [`README.md`](./README.md) first. This page explains how to demo it.

## Demo Pacing

1. Run one stage at a time
2. Print full output (do not summarize)
3. Explain what you see
4. Ask "Ready to continue?" before proceeding

## Stage 1: What Is This App?

```bash
cd spring-platform/springboot-platform-app-centric
cat deployment-map.json | jq
```

You'll see: `inventory-api` with three deployments (dev, stage, prod), each becoming a ConfigHub space.

**PAUSE.**

## Stage 2: Preview Setup

```bash
./setup.sh --explain
```

You'll see: ASCII diagram of App → Deployments → Targets, and the three mutation outcomes.

**PAUSE.**

## Stage 3: Create The Config

```bash
./setup.sh
cub space list --where "Labels.ExampleName = 'springboot-platform-app-centric'" --json | jq '.[].Space.Slug'
```

You'll see: 4 spaces created (3 env + 1 infra), each env space with a unit and noop target.

GUI checkpoint: ConfigHub → Spaces → filter `ExampleName=springboot-platform-app-centric`

**PAUSE.**

## Stage 4: Three Mutation Outcomes

```bash
./demo.sh
```

You'll see: apply-here, lift-upstream, and block/escalate routes with example fields.

**PAUSE.**

## Stage 5: Try A Mutation

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "demo: reservation mode strict → optimistic" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic

cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'
```

You'll see: mutation stored with audit trail.

GUI checkpoint: Open unit → History

**PAUSE.**

## Stage 6: Apply To Target

```bash
cub unit apply --space inventory-api-prod inventory-api
```

You'll see: unit applied to noop target (accepts but doesn't deliver).

**PAUSE.**

## Stage 7: Cleanup

```bash
./cleanup.sh
```

## Not Yet Implemented

- `lift upstream` automated PR
- `block/escalate` server-side enforcement
