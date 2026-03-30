# AI Start Here: Spring Boot Generator Example

Read [`README.md`](./README.md) first. This page explains how to demo it.

## Demo Pacing

1. Run one stage at a time
2. Print full output (do not summarize)
3. Explain what you see
4. Ask "Ready to continue?" before proceeding

## Stage 1: Preview

```bash
cd spring-platform/springboot-platform-app
./setup.sh --explain
./verify.sh
```

You'll see: the generator model, input/output paths, and the three mutation routes.

**PAUSE.**

## Stage 2: Generator Transformation

```bash
./generator/render.sh --explain
./generator/render.sh --trace
```

You'll see: how app inputs + platform policies become operational config, field by field.

Then show field lineage:

```bash
./generator/render.sh --explain-field spring.datasource.url
./generator/render.sh --explain-field feature.inventory.reservationMode
```

You'll see: `spring.datasource.url` is BLOCKED (platform-injected), `feature.inventory.reservationMode` is MUTABLE (app-owned).

**PAUSE.**

## Stage 3: ConfigHub Setup

```bash
./confighub-setup.sh --explain
./confighub-setup.sh
./confighub-verify.sh
```

You'll see: 3 spaces created (dev, stage, prod), each with one unit (`inventory-api`).

GUI checkpoint: ConfigHub → Spaces → filter `ExampleName=springboot-platform-app`

**PAUSE.**

## Stage 4: Apply-Here Mutation

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: reservation mode strict → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'
```

You'll see: the mutation stored with full audit trail.

GUI checkpoint: Open unit → History

**PAUSE.**

## Stage 5: Lift-Upstream Bundle

```bash
./generator/render.sh --explain-field spring.cache.type
./lift-upstream.sh --explain
./lift-upstream.sh --render-diff
```

You'll see: the Redis cache bundle showing exact changes needed in upstream inputs.

**PAUSE.**

## Stage 6: Block/Escalate Boundary

```bash
./generator/render.sh --explain-field spring.datasource.url
./block-escalate.sh --explain
./block-escalate.sh --render-attempt
```

You'll see: the boundary documentation. Server-side enforcement is not yet implemented.

**PAUSE.**

## Stage 7: Cleanup

```bash
./confighub-cleanup.sh
```

## Optional: Real Kubernetes

Requires Kind cluster:

```bash
./bin/create-cluster && ./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker
export KUBECONFIG=var/springboot-platform.kubeconfig WORKER_SPACE=springboot-infra
./confighub-setup.sh --with-targets
./verify-e2e.sh
./bin/teardown
```

## Not Yet Implemented

- `lift upstream` automated PR
- `block/escalate` server-side enforcement
