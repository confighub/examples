# Spring Boot Platform App v2 Live Plan

This document turns the current structural `springboot-platform-app` example
into a concrete v2 plan that we can run on ConfigHub plus GitHub without
changing the core story.

The rule is:

- keep the current example as `v1` structural proof
- add a `v2` live path for the same `inventory-api` service
- prove the same three routed outcomes against real state

## Goal

Make `inventory-api` a real Spring Boot service that we can:

- build and run locally
- call over HTTP and verify with tests
- represent in ConfigHub across `dev`, `stage`, and `prod`
- mutate in ConfigHub when the route is `apply here`
- project back to GitHub when the route is `lift upstream`
- reject when the route is `block/escalate`

## Why This Is Worth Doing

The current example is good at explaining the model, but it is still
structural-only.

v2 should let a user say:

- "This app is real."
- "I can call it and see values change."
- "I can see the same app in ConfigHub across multiple variants."
- "I can watch ConfigHub route a change correctly."

## Keep The Story The Same

Do not replace the current model.

Keep:

- one app: `inventory-api`
- one mutation system
- three outcomes:
  - `apply here`
  - `lift upstream`
  - `block/escalate`
- authority in ConfigHub
- provenance back to upstream app inputs and platform policy

## Make `inventory-api` Real

Add real Spring Boot source under [`upstream/app`](./upstream/app):

- `src/main/java/com/example/inventory/InventoryApiApplication.java`
- `src/main/java/com/example/inventory/api/InventoryController.java`
- `src/main/java/com/example/inventory/api/InventoryItem.java`
- `src/main/java/com/example/inventory/api/InventoryService.java`
- `src/test/java/com/example/inventory/api/InventoryControllerTest.java`

Also update [`upstream/app/pom.xml`](./upstream/app/pom.xml) so the app can be
built and tested normally.

## Minimum API Surface

The app should return real values, not just health checks.

Required endpoints:

- `GET /api/inventory/items`
- `GET /api/inventory/items/{sku}`
- `GET /api/inventory/summary`
- `GET /actuator/health`

Suggested response shape:

```json
{
  "service": "inventory-api",
  "environment": "prod",
  "reservationMode": "strict",
  "cacheBackend": "none",
  "items": [
    { "sku": "SKU-100", "name": "widget", "quantity": 12 },
    { "sku": "SKU-200", "name": "cable", "quantity": 4 }
  ]
}
```

The service should read a few values from config so the behavior is visible:

- `feature.inventory.reservationMode`
- `CACHE_BACKEND`
- `spring.application.name`

Use in-memory sample data so the first live version does not require a real
database.

That keeps the example runnable while still preserving the datasource boundary
as a platform-owned field.

## Variant Model

v2 should make `dev`, `stage`, and `prod` first-class.

Minimum variant differences:

- `dev`
  - `reservationMode=optimistic`
  - `CACHE_BACKEND=none`
- `stage`
  - `reservationMode=strict`
  - `CACHE_BACKEND=none`
- `prod`
  - `reservationMode=strict`
  - `CACHE_BACKEND=none`

Later, the `lift upstream` flow can change the durable cache contract so the
rendered operational shape for some variants uses `CACHE_BACKEND=redis`.

The app itself should expose the active values through
`GET /api/inventory/summary` so the variant is visible from one curl.

## ConfigHub Shape

Reuse existing repo patterns instead of inventing a new object model.

Follow:

- the multi-env shape from [`promotion-demo-data`](../../promotion-demo-data/)
- the ConfigHub-first flow shape from [`../global-app-layer`](../global-app-layer/)

The v2 setup should create:

- one app identity for `inventory-api`
- one deployment/variant identity each for `dev`, `stage`, and `prod`
- operational config objects in ConfigHub for each variant
- a visible activity trail for routed changes

## Three Live Proofs

### 1. `apply here`

Request:

- change prod `feature.inventory.reservationMode` from `strict` to
  `optimistic`

Proof:

- ConfigHub records the request and route
- ConfigHub stores the changed operational value
- the running `prod` app returns the new value from
  `GET /api/inventory/summary`
- a later refresh does not silently wipe the change

### 2. `lift upstream`

Request:

- add Redis-backed caching

Proof:

- the request starts in ConfigHub
- ConfigHub routes it to GitHub
- a PR updates upstream app inputs or platform inputs
- after merge and refresh, the operational shape changes in ConfigHub
- the running app now reports `CACHE_BACKEND=redis`

### 3. `block/escalate`

Request:

- mutate `spring.datasource.*` directly for one deployment

Proof:

- ConfigHub records the request
- the request is blocked or escalated
- no unsafe direct mutation lands in the operational config
- the running app stays unchanged

## GitHub Integration

The v2 flow should use a real GitHub repo, not just local fixture files.

The cleanest pattern is:

- keep the example source in this repo for documentation and local preview
- create or sync a small `inventory-api` GitHub repo for the live app inputs
- use the `lift upstream` route to create a PR against that repo

The first `lift upstream` proof does not need full automation.

Good enough for v2:

- ConfigHub creates a patch or suggested edit
- the example script opens or prints the exact GitHub branch/PR step
- the README shows the expected before/after evidence

Better follow-on:

- automate PR creation once the route semantics are stable

## Delivery Path

Start with a simple direct ConfigHub-to-cluster apply path before adding a full
GitOps controller requirement.

Recommended v2 phases:

1. **Code-first**
   Add the real Spring Boot app and local tests.
2. **Image-first**
   Build a container image and run it locally or in kind.
3. **ConfigHub-only**
   Materialize `dev`, `stage`, and `prod` operational config in ConfigHub.
4. **Live target**
   Bind a target and deploy one or more variants.
5. **Lift upstream**
   Prove a GitHub PR flow for Redis adoption.
6. **Optional GitOps follow-on**
   Add Argo or Flux only after the ConfigHub and GitHub story is already clear.

## Read-Only First Commands For v2

The future v2 example should still open with a safe preview path:

```bash
cd incubator/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq
./verify.sh
```

Then add a local code proof path:

```bash
cd upstream/app
mvn test
mvn spring-boot:run
curl -s http://localhost:8080/api/inventory/summary | jq
```

These should prove that `inventory-api` is callable before any ConfigHub or
cluster mutation happens.

## Success Criteria

v2 is successful when a new user can do all of the following:

- call `inventory-api` and get real JSON values back
- see `dev`, `stage`, and `prod` as distinct operational variants
- watch one `apply here` change affect a running variant
- watch one `lift upstream` request become a GitHub change and then a refreshed
  operational shape
- watch one `block/escalate` request get rejected cleanly
- explain the whole example as "one app, three requests, one activity log"

## Recommended First Implementation Slice

Build the smallest end-to-end slice first:

1. add the real Spring Boot API and tests
2. add a local run command and curl proof
3. materialize `dev`, `stage`, and `prod` in ConfigHub without a live cluster
4. prove one `apply here` mutation against ConfigHub state

Only after that should we add:

- cluster deployment
- GitHub PR creation
- optional Argo or Flux proof
