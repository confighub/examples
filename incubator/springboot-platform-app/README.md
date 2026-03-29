# Spring Boot generator example

## The Sequence

This is **#1** in a sequence of three related examples:

| # | Example | Focus |
|---|---------|-------|
| **1** | **This example** | Generator story (cub-gen): how app+platform → operational |
| 2 | [`springboot-platform-app-centric`](../springboot-platform-app-centric/) | App-centric: App → Deployments → Targets |
| 3 | [`springboot-platform-platform-centric`](../springboot-platform-platform-centric/) | Platform-centric: Platform → Apps → Deployments |

**Start here** if you want to understand the generator/authority story.
**Start with #2** for the app/deployment/target model.
**Start with #3** for multiple apps sharing one platform.

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

## What This Example Is For

Use this when the reason for the demo is to explain one real app across three mutation outcomes: apply here, lift upstream, and block or escalate.

This example exists to show that ConfigHub is not only an import or evidence tool. It can sit between app teams and platform teams as the decision point for real operational changes, and it now includes a real Kubernetes deployment path for the direct-apply case.

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

## The Generator

The generator is the transformation step that takes app inputs + platform policies and produces operational Kubernetes config:

```
upstream/app/           + upstream/platform/ → operational/
(Spring app inputs)       (Platform policies)   (Kubernetes manifests)
```

To see how this works:

```bash
./generator/render.sh --explain       # What the generator does
./generator/render.sh --trace         # Field-by-field mapping
```

Understanding the generator is key to understanding field ownership. When you know how a field got into `operational/deployment.yaml`, you know whether it's:
- **mutable-in-ch**: App-owned, safe to change locally in ConfigHub
- **lift-upstream**: App-owned, but durable changes should go back to source
- **generator-owned**: Platform-controlled, changes are blocked

See [`generator/README.md`](./generator/README.md) for the full transformation documentation.

## The Platform

The platform defines what capabilities apps get automatically and which fields are platform-controlled:

```bash
# See what the platform provides and controls
./generator/render.sh --platform-summary

# Check if a specific field is mutable
./generator/render.sh --explain-field spring.datasource.url
```

| Capability | What You Get | Platform Controls |
|------------|--------------|-------------------|
| Managed Datasource | PostgreSQL connection | HA, encryption, backups |
| Runtime Hardening | Secure defaults | runAsNonRoot, mTLS |
| Observability | Health endpoints | Alerting at SLO targets |

See [`docs/platform-onboarding.md`](./docs/platform-onboarding.md) for the full platform guide.

## What This Proves

This example has seven proof levels:

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
- the changed value is visible in stored ConfigHub config and can be replayed
  locally or delivered through the real deployment path below

### 4. Real Kubernetes deployment proof (`--with-targets` flag)

- deploys prod environment to a real Kubernetes cluster (local Kind)
- ConfigHub mutation → real `kubectl apply` via worker → real running pod
- HTTP verification hits the actual deployed app
- mutation changes are visible in the running application

Prerequisites for real deployment:
- Kind cluster (`./bin/create-cluster`)
- Docker image built (`./bin/build-image`)
- ConfigHub worker running (`./bin/install-worker`)

### 4b. Noop target proof (optional `--with-noop-targets` flag)

For simulation without a real cluster:
- shared infra space with a server worker (`inventory-api-infra`)
- Noop targets in each environment space (no real cluster needed)
- units are bound to targets and applied
- Noop worker accepts apply but does NOT deliver to Kubernetes
- unit status shows `Ready` / `Synced` / `ApplyCompleted`

### 5. Lift-upstream bundle proof (read-only)

- a deterministic Redis lift-upstream bundle exists for the same `inventory-api`
  story
- `./lift-upstream.sh --render-diff` prints the exact GitHub-ready patch for:
  - `upstream/app/pom.xml`
  - `upstream/app/src/main/resources/application.yaml`
  - refreshed ConfigHub YAMLs for dev, stage, and prod
- the bundle changes are concrete enough to review before any real PR is opened

### 6. Block/escalate boundary bundle (read-only)

- a concrete datasource override attempt exists for the same `inventory-api`
  story
- `./block-escalate.sh --render-attempt` prints the exact dry-run `cub function do`
  command for overriding `SPRING_DATASOURCE_URL`
- the route rules and runtime policy make the platform boundary explicit
- the current product state is classified honestly as "not yet proven" rather
  than pretending the field policy already exists

It does not yet prove:

- `lift upstream` via GitHub PR (diff bundle exists but no automated PR)
- `block/escalate` via field-level policy enforcement (boundary is documented, not server-enforced)

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

For real Kubernetes deployment (`--with-targets`), also:

- `kind` CLI (for local Kubernetes cluster)
- `kubectl`
- Docker (for building and running containers)

For Noop simulation (`--with-noop-targets`):

- No additional prerequisites (no real cluster needed)

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

What the real deployment scripts write (`--with-targets`):

- `./bin/create-cluster` creates a Kind cluster
- `./bin/build-image` builds and loads Docker image into Kind
- `./bin/install-worker` starts a ConfigHub worker process
- `./confighub-setup.sh --with-targets` deploys to the cluster
- `./bin/teardown` deletes the cluster and stops the worker

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
| [`upstream/platform/platform.yaml`](./upstream/platform/platform.yaml) | Consolidated platform manifest (provides + field routes) |
| [`upstream/platform/runtime-policy.yaml`](./upstream/platform/runtime-policy.yaml) | Platform-owned runtime policy |
| [`upstream/platform/slo-policy.yaml`](./upstream/platform/slo-policy.yaml) | Platform-owned SLO policy |
| [`generator/render.sh`](./generator/render.sh) | Shows how upstream inputs become operational config |
| [`generator/README.md`](./generator/README.md) | Documents the generator transformation |
| [`operational/configmap.yaml`](./operational/configmap.yaml) | Materialized operational config |
| [`operational/deployment.yaml`](./operational/deployment.yaml) | Materialized deployment shape |
| [`operational/service.yaml`](./operational/service.yaml) | Materialized service |
| [`operational/field-routes.yaml`](./operational/field-routes.yaml) | Route rules for field ownership and mutation behavior |
| [`changes/01-mutable-in-ch.md`](./changes/01-mutable-in-ch.md) | Direct ConfigHub mutation example |
| [`changes/02-lift-upstream.md`](./changes/02-lift-upstream.md) | Upstream routing example |
| [`changes/03-generator-owned.md`](./changes/03-generator-owned.md) | Block/escalate example |
| [`block-escalate.sh`](./block-escalate.sh) | Read-only datasource override boundary preview |
| [`block-escalate-verify.sh`](./block-escalate-verify.sh) | Verifies the block/escalate boundary bundle |
| [`lift-upstream.sh`](./lift-upstream.sh) | Read-only Redis lift-upstream preview and diff renderer |
| [`lift-upstream-verify.sh`](./lift-upstream-verify.sh) | Verifies the Redis lift-upstream bundle |
| [`confighub-setup.sh`](./confighub-setup.sh) | ConfigHub-only setup (creates spaces and units) |
| [`confighub-cleanup.sh`](./confighub-cleanup.sh) | ConfigHub-only cleanup |
| [`confighub-verify.sh`](./confighub-verify.sh) | ConfigHub-only verification |
| [`confighub/inventory-api-dev.yaml`](./confighub/inventory-api-dev.yaml) | Dev variant unit YAML |
| [`confighub/inventory-api-stage.yaml`](./confighub/inventory-api-stage.yaml) | Stage variant unit YAML |
| [`confighub/inventory-api-prod.yaml`](./confighub/inventory-api-prod.yaml) | Prod variant unit YAML |
| [`lift-upstream/redis-cache/upstream-app/pom.xml`](./lift-upstream/redis-cache/upstream-app/pom.xml) | Redis-ready upstream build input |
| [`lift-upstream/redis-cache/upstream-app/src/main/resources/application.yaml`](./lift-upstream/redis-cache/upstream-app/src/main/resources/application.yaml) | Redis-ready upstream app config |
| [`lift-upstream/redis-cache/confighub/inventory-api-dev.yaml`](./lift-upstream/redis-cache/confighub/inventory-api-dev.yaml) | Refreshed dev ConfigHub YAML after lift-upstream |
| [`lift-upstream/redis-cache/confighub/inventory-api-stage.yaml`](./lift-upstream/redis-cache/confighub/inventory-api-stage.yaml) | Refreshed stage ConfigHub YAML after lift-upstream |
| [`lift-upstream/redis-cache/confighub/inventory-api-prod.yaml`](./lift-upstream/redis-cache/confighub/inventory-api-prod.yaml) | Refreshed prod ConfigHub YAML after lift-upstream |
| [`docs/platform-onboarding.md`](./docs/platform-onboarding.md) | Platform onboarding guide for app teams |
| [`docs/platform-concept-design.md`](./docs/platform-concept-design.md) | Platform concept design rationale |

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
- [`lift-upstream.sh`](./lift-upstream.sh)

### 3. `block/escalate`

The app team tries to change `spring.datasource.*` or bypass the managed
datasource boundary.

That should be blocked or escalated because it is platform-owned. Fields get
blocked or escalated when they are platform-owned or generator-owned.

See:

- [`changes/03-generator-owned.md`](./changes/03-generator-owned.md)
- [`block-escalate.sh`](./block-escalate.sh)

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

## Lift-upstream Bundle Proof

The Redis caching request now has a read-only bundle that shows exactly what
would change upstream and in the refreshed ConfigHub YAMLs.

Use:

```bash
cd incubator/springboot-platform-app
./lift-upstream.sh --explain
./lift-upstream.sh --explain-json | jq
./lift-upstream.sh --render-diff
./lift-upstream-verify.sh
```

This does not mutate ConfigHub or Git. It is the concrete proof that the
`lift upstream` route can be turned into a GitHub-ready patch bundle for the
same `inventory-api` story.

## Block/escalate Boundary Bundle

The datasource boundary now has a concrete read-only artifact too.

Use:

```bash
cd incubator/springboot-platform-app
./block-escalate.sh --explain
./block-escalate.sh --explain-json | jq
./block-escalate.sh --render-attempt
./block-escalate-verify.sh
```

This does not mutate ConfigHub. It shows the exact dry-run datasource override
attempt that should eventually be blocked or escalated, while keeping the
current product boundary explicit: field-level policy enforcement is not yet
proven.

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

# Verify the app would see the new value (separate local replay, not live deployment)
cd upstream/app
FEATURE_INVENTORY_RESERVATIONMODE=optimistic mvn spring-boot:run -q -Dspring-boot.run.profiles=prod &
# then: curl -s http://localhost:8081/api/inventory/summary | jq .reservationMode
# expected: "optimistic"

# Clean up
./confighub-cleanup.sh
```

## Real Kubernetes Deployment

This is the **end-to-end proof**: ConfigHub mutation → real kubectl apply → real running pod → HTTP verification.

### Setup Infrastructure

```bash
# 1. Create Kind cluster
./bin/create-cluster

# 2. Build and load Docker image
./bin/build-image

# 3. Start ConfigHub worker (provides real Kubernetes target)
CUB_SPACE=springboot-infra ./bin/install-worker

# 4. Export KUBECONFIG for kubectl commands
export KUBECONFIG=var/springboot-platform.kubeconfig

# Optional: if install-worker printed an exact target slug, export it.
# confighub-setup.sh auto-detects the target when there is exactly one
# Kubernetes target in WORKER_SPACE.
# export K8S_TARGET=<printed-target-slug>
```

### Deploy and Verify

```bash
# 5. Setup ConfigHub + deploy prod to cluster
export WORKER_SPACE=springboot-infra
./confighub-setup.sh --with-targets

# 6. Verify deployment
./verify-e2e.sh
# → Shows: reservationMode = strict (from deployed pod)

# 7. Mutate
cub function do --space inventory-api-prod \
  --change-desc "rollout: reservation mode strict → optimistic" \
  -- set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

# 8. Re-apply (triggers real kubectl apply)
cub unit apply --space inventory-api-prod inventory-api

# 9. Wait for rollout
kubectl rollout status deployment/inventory-api -n inventory-api

# 10. Verify mutation
./verify-e2e.sh
# → Shows: reservationMode = optimistic (mutation is live!)

# 11. Cleanup
./confighub-cleanup.sh
./bin/teardown
```

### What This Proves

- ConfigHub mutation is real (stored in ConfigHub with audit trail)
- Apply is real (kubectl apply via worker to actual cluster)
- Deployment is real (pod running in Kind)
- Verification is real (HTTP call to actual deployed pod)

## Noop Target Proof (Simulation)

Use `--with-noop-targets` for simulation without a real cluster. This proves
the ConfigHub workflow but does NOT deploy to Kubernetes.

```bash
# Preview
./confighub-setup.sh --explain --with-noop-targets

# Create everything including Noop targets and apply
./confighub-setup.sh --with-noop-targets

# Verify including target status
./confighub-verify.sh --noop-targets

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

**Important:** Noop targets accept applies but do NOT deliver to any cluster.
The mutation is stored in ConfigHub but there is no running pod to verify.
Use `--with-targets` for real end-to-end proof.

## Alternative Delivery Modes: Flux OCI and Argo OCI

This example uses direct Kubernetes delivery (`kubectl apply` via worker). ConfigHub
also supports controller-based delivery where Flux or ArgoCD reconciles from an OCI
artifact published by ConfigHub.

| Delivery Mode | Provider Type | What Happens |
|--------------|---------------|--------------|
| Direct (this example) | `Kubernetes` | Worker applies via kubectl |
| Flux OCI | `FluxOCI` | Worker publishes to OCI, Flux reconciles |
| Argo OCI | `ArgoCDOCI` | Worker publishes to OCI, ArgoCD reconciles |

### Why Use Controller-Based Delivery?

David Flanagan's feedback: "The worker doing kubectl apply is a reconciler like Porch."

Controller-based delivery separates concerns:
- ConfigHub publishes the **artifact** (OCI bundle with manifests)
- Flux/ArgoCD does the **reconciliation** (continuous delivery to cluster)
- Clear separation between "what to deploy" and "how to deploy"

### Reference Examples

For full Flux OCI and Argo OCI implementation, see:

- [`global-app-layer/single-component`](../global-app-layer/single-component/) - One component with all three delivery variants
- [`global-app-layer/gpu-eks-h100-training`](../global-app-layer/gpu-eks-h100-training/) - Multi-component with Flux OCI

Key patterns from these examples:

1. **Separate deployment spaces per variant**: `{app}-deploy-cluster-a` (direct) vs `{app}-deploy-cluster-a-flux` (Flux)
2. **Clone units for each variant**: All variants clone from the same recipe unit
3. **Provider-type-aware target binding**: FluxOCI targets bind to Flux units
4. **Labels for filtering**: `DeliveryVariant=flux` or `DeliveryVariant=direct`

### Future: Adding Flux OCI to This Example

To add Flux OCI delivery to springboot-platform-app:

1. Create Flux variant spaces: `inventory-api-prod-flux`
2. Clone units from existing spaces: `inventory-api-flux` from `inventory-api`
3. Create FluxOCI target with OCI registry and Flux controller config
4. Bind Flux units to FluxOCI targets
5. Apply triggers OCI publish → Flux reconciles

This is tracked but not yet implemented. The mutation routing story (apply-here,
lift-upstream, block-escalate) is delivery-mode agnostic.

## What Is Not Yet Proven

- `lift upstream` via GitHub PR (diff bundle exists but no automated PR creation)
- `block/escalate` via field-level policy enforcement (boundary is documented, not server-enforced)
- Flux OCI or Argo OCI delivery (direct Kubernetes only; see global-app-layer for OCI examples)

If you want more:

- use `cub-gen` for source-to-operational provenance
- use `cub-scout` for live runtime inspection
- use the GitOps or layered examples in this repo for live ConfigHub flows

## Troubleshooting

If `jq` is missing:

- install `jq`
- or read [`example-summary.json`](./example-summary.json) directly

If you are not in the right directory:

- run `git rev-parse --show-toplevel`
- then `cd <repo-root>/incubator/springboot-platform-app`

If you expected a live cluster demo:

- use the "Real Kubernetes Deployment" section above
- this example now has a direct Kubernetes end-to-end path via `--with-targets`
- `--with-noop-targets` is the explicit simulation path, not the default

## Cleanup

The structural and local app proofs require no cleanup.

If you ran `./confighub-setup.sh`, clean up with:

```bash
./confighub-cleanup.sh
```

This deletes all spaces labeled `ExampleName=springboot-platform-app`.
