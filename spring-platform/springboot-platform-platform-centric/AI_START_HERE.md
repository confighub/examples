# AI Start Here: Spring Boot Platform (Experimental ADTP View)

Read [`README.md`](./README.md) first. This page explains how to demo it.

**Experimental:** Noop targets only in this example.

## Demo Pacing

1. Run one stage at a time
2. Print full output (do not summarize)
3. Explain what you see
4. Ask "Ready to continue?" before proceeding

## Stage 1: What Is This Platform?

```bash
cd spring-platform/springboot-platform-platform-centric
cat platform-map.json | jq
```

You'll see: `springboot-platform` with two apps (inventory-api, catalog-api) and which fields are platform-controlled.

**PAUSE.**

## Stage 2: Platform Capabilities

```bash
./platform.sh --summary
./platform.sh --apps
```

You'll see: managed datasource, runtime hardening, observability — and which apps inherit them.

**PAUSE.**

## Stage 3: Preview Setup

```bash
./setup.sh --explain
```

You'll see: 6 spaces (1 infra + 5 app), all with noop targets.

**PAUSE.**

## Stage 4: Create The Platform

```bash
./setup.sh
cub space list --where "Labels.Platform = 'springboot-platform'" --json | jq '.[].Space.Slug'
```

You'll see: all spaces tagged with the same platform label.

GUI checkpoint: ConfigHub → Spaces → filter `Platform=springboot-platform`

**PAUSE.**

## Stage 5: Field Ownership

```bash
./platform.sh --explain-field spring.datasource.url
./platform.sh --explain-field feature.inventory.reservationMode
```

You'll see: `spring.datasource.url` is platform-owned (blocked), `feature.inventory.reservationMode` is app-owned (mutable).

**PAUSE.**

## Stage 6: Mutate Both Apps

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "demo: enable optimistic reservation mode" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic

cub function do --space catalog-api-prod --unit catalog-api \
  --change-desc "demo: enable recommendations" \
  set-env catalog-api FEATURE_CATALOG_RECOMMENDATIONSENABLED=true

cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email}]'
```

You'll see: both mutations succeed (app-owned fields). Platform-owned fields would be blocked.

GUI checkpoint: Open each unit → History

**PAUSE.**

## Stage 7: Cleanup

```bash
./cleanup.sh
```

## Not Yet Implemented

- `lift upstream` automated PR
- `block/escalate` server-side enforcement
- Platform-as-resource in ConfigHub (label-based only)
