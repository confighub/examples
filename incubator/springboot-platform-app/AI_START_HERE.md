# AI Start Here

Use this page when you want to drive `springboot-platform-app` safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. ask `Ready to continue?`
6. wait for the human before continuing

This example has several mutation-routing scenarios. For each one, explain:

- the pain
- the fix
- what you see
- what this proves
- what this does not prove

## Suggested Prompt

```text
Read incubator/springboot-platform-app/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Example Is For

This is a structural Spring Boot platform/app example for the authority versus provenance model, with a locally runnable upstream app.

It demonstrates one app, `inventory-api`, with three routed outcomes:

- `apply here`
- `lift upstream`
- `block/escalate`

## Proof Types

This example has seven proof levels:

1. structural: fixture files and contracts
2. local app: Spring Boot HTTP tests
3. ConfigHub-only: real spaces and units
4. **real deployment**: Kind cluster + real kubectl apply + HTTP verification
5. noop target: apply workflow with Noop targets (simulation)
6. lift-upstream bundle: read-only Redis patch bundle
7. block/escalate boundary: read-only datasource override attempt

It does not yet:

- create a real GitHub PR for lift-upstream
- prove actual `block/escalate` enforcement in ConfigHub (boundary is documented, not server-enforced)

## Stage 1: Preview The Structure (read-only)

```bash
cd incubator/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq
./verify.sh
```

These commands do not mutate ConfigHub or live infrastructure.

GUI checkpoint:

- none yet; this stage is CLI-only preview of the model

Pause after this stage.

## Stage 2: Understand The Generator (read-only)

The generator is the transformation step that takes app inputs + platform policies and produces operational Kubernetes config.

```bash
./generator/render.sh --explain
./generator/render.sh --trace
```

What you should see:

- The 5 key transformations (app name extraction, datasource injection, port mapping, config embedding, profile activation)
- Field-by-field mapping from inputs to outputs
- Which fields are generator-owned (platform-controlled) vs app-owned

Why this matters:

Understanding the generator is key to understanding field ownership. When you know how a field got into `operational/deployment.yaml`, you know whether it's mutable-in-ch, lift-upstream, or generator-owned (blocked).

GUI checkpoint:

- none; this stage is CLI-only

Pause after this stage.

## Stage 3: Prove The Local App Works (local only)

```bash
cd upstream/app
mvn test
```

What this writes:

- local build output only

What you should see after:

- passing HTTP-oriented tests for the local Spring Boot app

GUI checkpoint:

- none; this stage is local-only

Pause after this stage.

## Stage 4: Create ConfigHub Structure (mutates ConfigHub)

```bash
cd ..
./confighub-setup.sh --explain
./confighub-setup.sh
./confighub-verify.sh
```

What this mutates:

- real ConfigHub spaces and units for dev, stage, and prod

What you should see after:

- spaces and units for `inventory-api-*`
- verification output showing the expected units and relationships

GUI checkpoint:

- ConfigHub GUI: open the spaces and units for the newly created `inventory-api-*` objects

Pause after this stage.

## Stage 5: Apply-Here Mutation Proof (mutates ConfigHub)

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: override reservationMode from strict to optimistic for prod rollout" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
```

What you see after:

- the prod unit shows the env override in ConfigHub
- a separate local replay can be run with the same env override

Verification:

```bash
cd upstream/app
FEATURE_INVENTORY_RESERVATIONMODE=optimistic mvn spring-boot:run -q -Dspring-boot.run.profiles=prod
```

GUI checkpoint:

- ConfigHub GUI: open the prod unit and inspect the changed env value

Pause after this stage.

## Stage 6: Real Kubernetes Deployment (mutates cluster)

This is the **end-to-end proof**: ConfigHub → real kubectl apply → running pod → HTTP verification.

### Prerequisites (run once)

```bash
cd incubator/springboot-platform-app
./bin/create-cluster
./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker
export KUBECONFIG=var/springboot-platform.kubeconfig
# Optional: export K8S_TARGET=<printed-target-slug> if install-worker reported one.
# confighub-setup.sh auto-detects it when there is exactly one Kubernetes target.
```

### Deploy and Verify

```bash
export WORKER_SPACE=springboot-infra
./confighub-setup.sh --with-targets
./verify-e2e.sh
```

What you should see after:

- Pod running in Kind cluster
- HTTP response from the actual deployed app
- `reservationMode = strict` (the default)

### Mutate and Verify

```bash
cub function do --space inventory-api-prod \
  --change-desc "rollout: reservation mode strict → optimistic" \
  -- set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
cub unit apply --space inventory-api-prod inventory-api
kubectl rollout status deployment/inventory-api -n inventory-api
./verify-e2e.sh
```

What you should see after:

- `reservationMode = optimistic` (mutation is live!)

What this proves:

- ConfigHub mutation → real kubectl apply → real running pod
- Verification via actual HTTP call to deployed app
- No simulation, no Noop targets

GUI checkpoint:

- ConfigHub GUI: inspect the prod unit and its mutation history
- kubectl: `kubectl get pods -n inventory-api`

Pause after this stage.

## Stage 6b: Noop Target Workflow (simulation, no real cluster)

For simulation without a real cluster, use `--with-noop-targets`.

```bash
./confighub-cleanup.sh  # Clean up from real deployment first
./confighub-setup.sh --with-noop-targets
./confighub-verify.sh --noop-targets
```

What this proves:

- the apply workflow can be exercised without a real cluster

What this does NOT prove:

- no actual Kubernetes delivery (Noop targets accept but don't deploy)
- no HTTP verification (there's no running pod)

GUI checkpoint:

- ConfigHub GUI: inspect targets, bindings, and apply-related status

Pause after this stage.

## Stage 7: Lift-Upstream Bundle (read-only)

```bash
./lift-upstream.sh --explain
./lift-upstream.sh --explain-json | jq
./lift-upstream.sh --render-diff
./lift-upstream-verify.sh
```

What this proves:

- the exact upstream app and ConfigHub changes needed for the Redis caching request can be rendered read-only

GUI checkpoint:

- GUI equivalent is not built here; the rendered diff is the source of truth

Pause after this stage.

## Stage 8: Block/Escalate Boundary (read-only)

```bash
./block-escalate.sh --explain
./block-escalate.sh --explain-json | jq
./block-escalate.sh --render-attempt
./block-escalate-verify.sh
```

What this proves:

- the datasource override attempt is visible and explainable

What this does not prove:

- enforcement is still a product gap and is not claimed here

GUI checkpoint:

- GUI equivalent is not built here; use the dry-run output as the evidence

Pause after this stage.

## Stage 9: Cleanup

```bash
./confighub-cleanup.sh
./bin/teardown  # If you ran real deployment
```

## Follow-On

If the human wants a live next step:

- use the GitOps import examples for cluster-first flows
- use `global-app-layer` for ConfigHub-first layered flows
- use `cub-gen` plus `cub-scout` when the question is provenance plus runtime
