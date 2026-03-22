# Spring Boot generator example

Example is a Heroku-like Spring Boot "platform" service such as
`inventory-api`, with `dev`, `stage`, and `prod` deployments.

Git still holds code and upstream Spring inputs. This includes
`application.yaml`. The platform renders those inputs into operational config
and deployment shape. App owners should not see the platform machinery, but
they can use ConfigHub.

ConfigHub is where operational config becomes authoritative, mutable, and
inspectable.

It is the mutation plane: the place where operational changes are requested,
decided, and routed.

## Stack And Scenario

This example is for:

- Spring Boot app teams
- platform teams that render app inputs into operational config
- ConfigHub users who need to explain which changes should mutate in ConfigHub,
  which should be lifted upstream, and which must stay platform-owned

The app is `inventory-api`.

The main question it answers is:

"When I need to change this running app, should that be a direct ConfigHub
mutation, a change routed back to the Spring app inputs, or a platform-owned
field that gets blocked or escalated?"

Once platform-rendered operational config is in ConfigHub, every governed
change request enters there. ConfigHub records the request, provenance, and
decision, then routes it: apply here, lift upstream, or block/escalate.

## What This Proves

This is a structural ConfigHub proof with a locally runnable Spring Boot app.

It proves:

- a realistic split between app inputs and platform policy
- a real `inventory-api` upstream app with HTTP-level tests
- a clear operational shape that ConfigHub could store authoritatively
- one mutation system with three outcomes:
  - `apply here`
  - `lift upstream`
  - `block/escalate`

It does not prove:

- a live cluster apply
- a worker or target integration
- a full end-to-end ConfigHub setup flow

## Prerequisites

You need:

- `bash`
- `jq`

If you want to run the local app or its HTTP tests, you also need:

- Java 21+
- Maven

You do not need:

- `cub`
- `cub auth login`
- a worker
- a target
- a cluster

## What This Reads And Writes

What it reads:

- upstream app inputs under [`upstream/app`](./upstream/app)
- upstream platform policy under [`upstream/platform`](./upstream/platform)
- operational shape under [`operational`](./operational)
- example metadata in [`example-summary.json`](./example-summary.json)

What it writes:

- nothing in ConfigHub
- nothing in a cluster
- nothing outside this directory

The provided scripts are read-only.

## Read-Only Preview

Start here:

```bash
cd incubator/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub or live infrastructure.

## File Layout

| Path | Purpose |
|---|---|
| [`upstream/app/pom.xml`](./upstream/app/pom.xml) | App-owned Spring Boot build input |
| [`upstream/app/src/main/java/com/example/inventory/InventoryApiApplication.java`](./upstream/app/src/main/java/com/example/inventory/InventoryApiApplication.java) | Spring Boot entrypoint |
| [`upstream/app/src/main/java/com/example/inventory/api/InventoryController.java`](./upstream/app/src/main/java/com/example/inventory/api/InventoryController.java) | HTTP API controller |
| [`upstream/app/src/main/java/com/example/inventory/api/InventoryService.java`](./upstream/app/src/main/java/com/example/inventory/api/InventoryService.java) | In-memory inventory service |
| [`upstream/app/src/main/resources/application.yaml`](./upstream/app/src/main/resources/application.yaml) | Base Spring config |
| [`upstream/app/src/main/resources/application-stage.yaml`](./upstream/app/src/main/resources/application-stage.yaml) | Stage app overrides |
| [`upstream/app/src/main/resources/application-prod.yaml`](./upstream/app/src/main/resources/application-prod.yaml) | Prod app overrides |
| [`upstream/app/src/test/java/com/example/inventory/api/InventoryControllerHttpTest.java`](./upstream/app/src/test/java/com/example/inventory/api/InventoryControllerHttpTest.java) | HTTP-level test for default profile |
| [`upstream/app/src/test/java/com/example/inventory/api/InventoryControllerProdHttpTest.java`](./upstream/app/src/test/java/com/example/inventory/api/InventoryControllerProdHttpTest.java) | HTTP-level test for prod profile |
| [`upstream/platform/runtime-policy.yaml`](./upstream/platform/runtime-policy.yaml) | Platform-owned runtime policy |
| [`upstream/platform/slo-policy.yaml`](./upstream/platform/slo-policy.yaml) | Platform-owned SLO policy |
| [`operational/configmap.yaml`](./operational/configmap.yaml) | Materialized operational config |
| [`operational/deployment.yaml`](./operational/deployment.yaml) | Materialized deployment shape |
| [`operational/service.yaml`](./operational/service.yaml) | Materialized service |
| [`operational/field-routes.yaml`](./operational/field-routes.yaml) | Route rules for field ownership and mutation behavior |
| [`changes/01-mutable-in-ch.md`](./changes/01-mutable-in-ch.md) | Direct ConfigHub mutation example |
| [`changes/02-lift-upstream.md`](./changes/02-lift-upstream.md) | Upstream routing example |
| [`changes/03-generator-owned.md`](./changes/03-generator-owned.md) | Block/escalate example |

## The Three Outcomes

Given one app, there is one mutation system with three outcomes:

### 1. `apply here`

Prod rollout change: `feature.inventory.reservationMode=strict -> optimistic`
for `inventory-api-prod`.

This is a direct operational mutation in ConfigHub. This change should survive
normal refreshes from upstream.

See:

- [`changes/01-mutable-in-ch.md`](./changes/01-mutable-in-ch.md)

### 2. `lift upstream`

The service now needs Redis caching.

The intent may start in ConfigHub, but a few durable changes belong in the
Spring app inputs and source repo because the platform-rendered shape must
itself evolve. The machinery can support mutations that get lifted upstream,
including later MR/PR linkage.

See:

- [`changes/02-lift-upstream.md`](./changes/02-lift-upstream.md)

### 3. `block/escalate`

The app team tries to change `spring.datasource.*` or bypass the managed
datasource boundary.

That should be blocked or escalated because it is platform-owned. Fields get
blocked or escalated when they are platform-owned or generator-owned.

See:

- [`changes/03-generator-owned.md`](./changes/03-generator-owned.md)

## Local API Proof

The upstream Spring Boot app is now real enough to call and test locally.

Use:

```bash
cd incubator/springboot-platform-app/upstream/app
mvn test
```

These tests start the app on a random local port and call the HTTP API. They
do not mutate ConfigHub or live infrastructure.

To run the app manually:

```bash
cd incubator/springboot-platform-app/upstream/app
mvn spring-boot:run
curl -s http://localhost:8080/api/inventory/summary | jq
curl -s http://localhost:8080/api/inventory/items | jq
```

The main callable endpoints are:

- `GET /api/inventory/items`
- `GET /api/inventory/items/{sku}`
- `GET /api/inventory/summary`
- `GET /actuator/health`

## ConfigHub as authority tracking provenance

We distinguish authority vs provenance.

- Authority: where operational config is authoritatively mutated and managed.
- Provenance: causal trace from mutations to upstream code, generators,
  scaffolds, or external systems that produced or influenced that config.

A generator maps code to config plus provenance, so that given the latter, the
former may be traced.

This makes generated config tractable at scale: one place to see requested
changes, provenance, ownership, campaigns, and outcomes across many apps and
deployments.

- ConfigHub is the system of record for operational config and its mutation
  history; authoritative, mutable, and inspectable.
- deployments have targets and live state
- config may have upstream producers or sources
- Git remains the home of code, templates, generators, and other upstream
  producers

These are compatible if correctly managed. We can have tools that trace flow
from upstream producers, and govern the path from DRY inputs to operational
config. We can have tools that help us understand what actually happened, if
live reality is opaque.

The enemy is opaque, uncontrolled Git-centric config sprawl.

## Config sprawl and copied config

Sprawl is real. Multiple remote stores may generate, sync, or project local
copies of config:

- Git repos
- secrets systems
- cloud control planes
- SaaS tools
- platform generators
- local rendered or cached copies

That can still work if ConfigHub treats those copies as projections with
provenance, instead of pretending they do not exist or letting them become
silent competing sources of truth.

To make that work:

- record where each projection came from and when
- route each requested change through ConfigHub
- classify each field as `apply here`, `lift upstream`, or `block/escalate`
- merge refreshes by policy instead of blind overwrite
- show drift and divergence explicitly
- keep one activity log in ConfigHub even when the durable edit lands elsewhere

## Mutations to generated config in ConfigHub

This is the key case, and it is not a contradiction.

Generated or platform-rendered operational config can live in ConfigHub and
still be mutated in ConfigHub. The important thing is that future refreshes are
policy-driven, not blind overwrite.

Each field needs one of three behaviors:

- `apply here`: edit directly in ConfigHub; the change survives future refreshes
- `lift upstream`: route the durable change back to the generator input or source repo
- `block/escalate`: stop or route the request because the field is not safe to diverge locally

So the bad model is:

- regenerate into ConfigHub and wipe whatever was there

The good model is:

- treat regeneration as another mutation source
- merge it against existing ConfigHub state using ownership and invariants
- preserve, lift, or reject changes explicitly

Example to show: "Here is what is running, where it came from, what changed,
and the next safe mutation."

One app, three requests, one activity log:

- config edit rollout: apply here
- add Redis: lift upstream
- change datasource boundary: block/escalate

## Exact CLI Sequence

```bash
cd incubator/springboot-platform-app

# Human-readable preview
./setup.sh --explain

# Machine-readable contract
./setup.sh --explain-json | jq

# Static consistency checks
./verify.sh
```

All three commands are read-only.

## Expected Output

After `./setup.sh --explain`, you should see:

- the stack and scenario
- proof type: `structural`
- note that the upstream app can be tested over HTTP with `mvn test`
- the three behavior categories
- the exact files this example uses

After `./setup.sh --explain-json | jq`, you should see:

- `proof_type: "structural"`
- `mutates_confighub: false`
- `mutates_live_infra: false`
- three behavior entries named:
  - `apply_here`
  - `lift_upstream`
  - `block_or_escalate`

After `./verify.sh`, you should see:

- `ok: springboot-platform-app fixtures are consistent`

If Maven and Java are installed, after `mvn test` you should see:

- the Spring Boot tests start a local HTTP server
- `InventoryControllerHttpTest` exercise `/api/inventory/items` and `/api/inventory/summary`
- `InventoryControllerProdHttpTest` exercise `/api/inventory/summary` with the `prod` profile

## Verify It

```bash
./setup.sh --explain-json | jq '.behaviors[].name'
./setup.sh --explain-json | jq '.reads'
./verify.sh
```

Optional local app proof:

```bash
cd upstream/app
mvn test
```

## Inspect It In The GUI

There is no GUI or live ConfigHub path for this example yet.

This example is the crisp conceptual starting point, not the live wedge.

If you want the next step after understanding this model:

- use `cub-gen` for source-to-operational provenance
- use `cub-scout` for live runtime inspection
- use the GitOps or layered examples in this repo for live ConfigHub flows
- use [`V2-LIVE-PLAN.md`](./V2-LIVE-PLAN.md) for the concrete next step that
  turns this same `inventory-api` story into a runnable ConfigHub + GitHub
  example with a callable API

## Troubleshooting

If `jq` is missing:

- install `jq`
- or read [`example-summary.json`](./example-summary.json) directly

If you are not in the right directory:

- run `git rev-parse --show-toplevel`
- then `cd <repo-root>/incubator/springboot-platform-app`

If you expected a live demo:

- this example is intentionally structural and read-only
- use it to explain the model before moving to live GitOps or layered examples
- the concrete follow-on for this same service is
  [`V2-LIVE-PLAN.md`](./V2-LIVE-PLAN.md)

## Cleanup

No cleanup is required.

This example does not create ConfigHub objects or touch live infrastructure.
