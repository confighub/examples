# AI Start Here: Spring Boot Generator Example

This page helps AI assistants guide humans through this example.

Read the [`README.md`](./README.md) first. It explains the model. This page
explains how to demo it.

## Demo Pacing Rules

Pause after every stage:

1. Run only that stage's commands
2. Print the full output (do not summarize)
3. Explain what the output means
4. Print GUI checkpoints where applicable
5. Ask "Ready to continue?" and wait

## Suggested Prompt

```text
Read spring-platform/springboot-platform-app/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Explain how app inputs and platform policy become operational config.
Do not continue until I say continue.
```

## Stage 1: Preview (read-only)

```bash
cd spring-platform/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq
./verify.sh
```

What to explain:
- The stack and scenario
- What the structural scripts read
- The three mutation routes

GUI checkpoint: none (CLI preview only)

**PAUSE.**

## Stage 2: Generator Transformation (read-only)

```bash
./generator/render.sh --explain
./generator/render.sh --trace
```

What to explain:
- How app inputs + platform policies become operational config
- The 5 key transformations
- Field-by-field mapping

Then show field lineage:

```bash
./generator/render.sh --explain-field spring.datasource.url
./generator/render.sh --explain-field feature.inventory.reservationMode
```

What to explain:
- `spring.datasource.url` is BLOCKED (generator injects from platform)
- `feature.inventory.reservationMode` is MUTABLE (comes from app inputs)

GUI checkpoint: none

**PAUSE.**

## Stage 3: Local App Proof (optional, requires Java)

```bash
cd upstream/app
mvn test
cd ../..
```

What to explain:
- The Spring Boot app starts in tests
- HTTP tests call the API
- Default and prod profile responses are observable

GUI checkpoint: none

**PAUSE.**

## Stage 4: ConfigHub Setup (mutates ConfigHub)

```bash
./confighub-setup.sh --explain
./confighub-setup.sh
./confighub-verify.sh
```

What to explain:
- Creates 3 spaces: dev, stage, prod
- Each space contains one unit: `inventory-api`
- Labels enable filtering and cleanup

GUI checkpoint: Open ConfigHub GUI → Spaces → filter by `ExampleName=springboot-platform-app`

**PAUSE.**

## Stage 5: Apply-Here Mutation (mutates ConfigHub)

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: reservation mode strict → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
```

Then show the mutation history:

```bash
cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'
```

What to explain:
- The mutation is stored in ConfigHub
- The audit trail records who, when, and why
- This mutation survives future refreshes

GUI checkpoint: Open unit → History → see the mutation

**PAUSE.**

## Stage 6: Real Kubernetes Deployment (optional, requires Kind)

### Prerequisites

```bash
./bin/create-cluster
./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker
export KUBECONFIG=var/springboot-platform.kubeconfig
export WORKER_SPACE=springboot-infra
```

### Deploy and Verify

```bash
./confighub-setup.sh --with-targets
./verify-e2e.sh
```

What to explain:
- ConfigHub mutation → real kubectl apply → running pod
- HTTP verification hits the actual deployed app
- No simulation

### Mutate and Verify

```bash
cub function do --space inventory-api-prod \
  --change-desc "rollout: reservation mode strict → optimistic" \
  -- set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
cub unit apply --space inventory-api-prod inventory-api
kubectl rollout status deployment/inventory-api -n inventory-api
./verify-e2e.sh
```

GUI checkpoint: kubectl get pods -n inventory-api

**PAUSE.**

## Stage 7: Lift-Upstream Bundle (read-only)

```bash
./generator/render.sh --explain-field spring.cache.type
./lift-upstream.sh --explain
./lift-upstream.sh --render-diff
./lift-upstream-verify.sh
```

What to explain:
- Field lineage shows why this routes to lift-upstream
- The bundle shows exact changes to upstream inputs
- The bundle shows refreshed ConfigHub YAMLs
- No automated PR yet

GUI checkpoint: none

**PAUSE.**

## Stage 8: Block/Escalate Boundary (read-only)

```bash
./generator/render.sh --explain-field spring.datasource.url
./block-escalate.sh --explain
./block-escalate.sh --render-attempt
./block-escalate-verify.sh
```

What to explain:
- Field lineage shows why this is blocked
- The boundary is documented
- Server-side enforcement is not yet implemented

GUI checkpoint: none

**PAUSE.**

## Stage 9: Cleanup

```bash
./confighub-cleanup.sh
./bin/teardown  # if you ran real deployment
```

## What This Does Not Prove

- `lift upstream` via automated GitHub PR
- `block/escalate` via server-side enforcement
- Flux/Argo delivery (see `global-app-layer` examples)
