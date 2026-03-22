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

This example has three proof levels:

### 1. Structural proof

- a realistic split between app inputs and platform policy
- a clear operational shape that ConfigHub could store authoritatively
- one mutation system with three outcomes:
  - `apply here`
  - `lift upstream`
  - `block/escalate`

### 2. Local app proof

- a real `inventory-api` upstream app with HTTP-level tests
- `mvn test` starts the app on a random port and calls the HTTP API
- default and prod profile responses are observable over HTTP

### 3. ConfigHub-only proof

- real ConfigHub spaces and units for `inventory-api-dev`, `inventory-api-stage`, `inventory-api-prod`
- per-environment operational config stored in ConfigHub
- `apply here` mutation proven via `FEATURE_INVENTORY_RESERVATIONMODE` env var
  on the prod Deployment, which Spring Boot maps to
  `feature.inventory.reservationMode` via relaxed binding
- the running app reports the changed value via `/api/inventory/summary`

### 4. Noop target proof (optional `--with-targets` flag)

- shared infra space with a server worker (`inventory-api-infra`)
- Noop targets in each environment space (no real cluster needed)
- units are bound to targets and applied
- `apply here` mutation survives re-apply: the env var override persists
  after the unit is applied to the Noop target
- unit status shows `Ready` / `Synced` / `ApplyCompleted`

It does not yet prove:

- real Kubernetes cluster delivery
- `lift upstream` via GitHub PR
- `block/escalate` via field-level policy enforcement

## Prerequisites

For structural proof:

- `bash`
- `jq`

For local app proof, also:

- Java 21+
- Maven

For ConfigHub-only proof, also:

- `cub` CLI
- `cub auth login` (authenticated context)

You do not need:

- a Kubernetes cluster (Noop targets are used for the apply proof)

## What This Reads And Writes

What it reads:

- upstream app inputs under [`upstream/app`](./upstream/app)
- upstream platform policy under [`upstream/platform`](./upstream/platform)
- operational shape under [`operational`](./operational)
- example metadata in [`example-summary.json`](./example-summary.json)

What the structural scripts write:

- nothing in ConfigHub
- nothing in a cluster
- nothing outside this directory

What the ConfigHub-only scripts write:

- `./confighub-setup.sh` creates spaces and units in ConfigHub
- `./confighub-cleanup.sh` deletes them
- no cluster or live infrastructure is touched

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
| [`confighub-setup.sh`](./confighub-setup.sh) | ConfigHub-only setup (creates spaces and units) |
| [`confighub-cleanup.sh`](./confighub-cleanup.sh) | ConfigHub-only cleanup |
| [`confighub-verify.sh`](./confighub-verify.sh) | ConfigHub-only verification |
| [`confighub/inventory-api-dev.yaml`](./confighub/inventory-api-dev.yaml) | Dev variant unit YAML |
| [`confighub/inventory-api-stage.yaml`](./confighub/inventory-api-stage.yaml) | Stage variant unit YAML |
| [`confighub/inventory-api-prod.yaml`](./confighub/inventory-api-prod.yaml) | Prod variant unit YAML |

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
- the three proof levels (structural, local app, ConfigHub-only)
- what the structural scripts read and write
- safe next steps including `./confighub-setup.sh --explain`

After `./setup.sh --explain-json | jq`, you should see:

- `proof_type: "structural+confighub-only"`
- `mutates_confighub: false` (for the structural scripts)
- `confighub_only.mutates_confighub: true` (for the ConfigHub scripts)
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

## ConfigHub-Only Proof

This example includes a ConfigHub-only setup path that creates real ConfigHub
objects without requiring a cluster.

```bash
# Preview what will be created (read-only)
./confighub-setup.sh --explain
./confighub-setup.sh --explain-json | jq

# Create ConfigHub objects (mutating)
./confighub-setup.sh

# Verify ConfigHub objects exist
./confighub-verify.sh

# Inspect created objects
cub space list --where "Labels.ExampleName = 'springboot-platform-app'" --json
cub unit get --space inventory-api-prod --json inventory-api

# Prove apply-here mutation: override reservationMode from strict to optimistic
# FEATURE_INVENTORY_RESERVATIONMODE maps to feature.inventory.reservationMode
# via Spring Boot relaxed binding
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: override reservationMode from strict to optimistic for prod rollout" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

# Verify the mutation is stored in ConfigHub
cub function do --space inventory-api-prod --unit inventory-api \
  --output-only yq 'select(.kind == "Deployment") | .spec.template.spec.containers[0].env'

# Verify the app would see the new value (local proof)
cd upstream/app
FEATURE_INVENTORY_RESERVATIONMODE=optimistic mvn spring-boot:run -q -Dspring-boot.run.profiles=prod &
# then: curl -s http://localhost:8081/api/inventory/summary | jq .reservationMode
# expected: "optimistic"

# Clean up
./confighub-cleanup.sh
```

## Noop Target Proof

Add `--with-targets` to also create a server worker, Noop targets, bind units,
and apply them. No real cluster required.

```bash
# Preview
./confighub-setup.sh --explain --with-targets

# Create everything including targets and apply
./confighub-setup.sh --with-targets

# Verify including target status
./confighub-verify.sh --targets

# Mutate prod, then re-apply — mutation survives
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: override reservationMode from strict to optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
cub unit apply --space inventory-api-prod inventory-api

# Inspect
cub unit get --space inventory-api-prod --json inventory-api | jq '.UnitStatus'

# Clean up
./confighub-cleanup.sh
```

## What Is Not Yet Proven

- real Kubernetes cluster delivery (Noop targets accept but do not deploy)
- `lift upstream` via GitHub PR
- `block/escalate` via field-level policy enforcement

If you want the next step after this ConfigHub-only proof:

- use `cub-gen` for source-to-operational provenance
- use `cub-scout` for live runtime inspection
- use the GitOps or layered examples in this repo for live ConfigHub flows
- use [`V2-LIVE-PLAN.md`](./V2-LIVE-PLAN.md) for the full v2 plan

## Troubleshooting

If `jq` is missing:

- install `jq`
- or read [`example-summary.json`](./example-summary.json) directly

If you are not in the right directory:

- run `git rev-parse --show-toplevel`
- then `cd <repo-root>/incubator/springboot-platform-app`

If you expected a live cluster demo:

- this example proves structural, local app, and ConfigHub-only levels
- it does not yet prove live cluster apply or GitOps integration
- the concrete follow-on for this same service is
  [`V2-LIVE-PLAN.md`](./V2-LIVE-PLAN.md)

## Cleanup

The structural and local app proofs require no cleanup.

If you ran `./confighub-setup.sh`, clean up with:

```bash
./confighub-cleanup.sh
```

This deletes all spaces labeled `ExampleName=springboot-platform-app`.
