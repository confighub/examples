# AI Start Here: The Release-Day Challenge

Read [`README.md`](./README.md) first. It frames the scenario. This page is for running the challenge yourself or pairing with an AI assistant.

## Fast Path

If you already know Spring Boot and Kubernetes, run this first:

```bash
cub context list --json
cub space list --json

cd spring-platform/springboot-platform-app
./setup.sh --explain
./verify.sh
./generator/render.sh --trace
./generator/render.sh --explain-field feature.inventory.reservationMode
./generator/render.sh --explain-field spring.datasource.url
```

Then jump to the real generator example:

```bash
git clone https://github.com/confighub/cub-gen.git
cd cub-gen
go build -o ./cub-gen ./cmd/cub-gen
./examples/springboot-paas/demo-local.sh
./examples/springboot-paas/demo-governed-routes.sh
./examples/springboot-paas/demo-embedded-config-mutation.sh

# When you want the connected ConfigHub path
cub auth login
./examples/springboot-paas/demo-connected.sh
```

If `cub space list --json` fails because auth is missing or expired, run `cub auth login` before any mutating stage. For the `cub-gen` bridge, `demo-local.sh` does not need auth; `demo-connected.sh` does.

## The Scenario

A product launch is in 24 hours. The team needs three changes:

1. Flip `feature.inventory.reservationMode` to `optimistic` in prod (safe, urgent)
2. Add Redis caching (requires source changes)
3. Change the staging datasource (must be refused)

You will walk through all three using three views of the same model.

## Pairing Modes

Use whichever mode fits the person at the keyboard:

- Solo evaluation: run a full phase, then stop and summarize what mattered.
- Guided walkthrough: run one stage at a time, show full output, and pause at the marked checkpoints.

## Suggested Prompts

```text
Read spring-platform/AI_START_HERE.md and walk me through the release-day challenge.
Pause after every stage. Show full output. Do not continue until I say continue.
```

```text
Read spring-platform/AI_START_HERE.md and help me evaluate the release-day challenge quickly.
Use the fast path first, summarize what matters after each phase, and only pause when I ask.
```

## Phase 1: How Config Gets Generated (springboot-platform-app)

### Stage 1.1: Preview (read-only)

```bash
cd spring-platform/springboot-platform-app
./setup.sh --explain
./verify.sh
```

You'll see the stack, the three mutation routes, and what would be created in ConfigHub.

Pause here if you're doing a guided walkthrough.

### Stage 1.2: Generator Trace (read-only)

```bash
./generator/render.sh --trace
./generator/render.sh --explain-field feature.inventory.reservationMode
./generator/render.sh --explain-field spring.datasource.url
```

You'll see every field mapped from input to output. The first field is MUTABLE (app-owned). The second is BLOCKED (platform policy injects it).

Pause here if you're doing a guided walkthrough.

### Stage 1.3: Create ConfigHub Objects (mutates ConfigHub)

Ask: "This will create 3 spaces and 3 units. OK?"

```bash
./confighub-setup.sh
./confighub-verify.sh
```

You'll see three spaces created: `inventory-api-dev`, `-stage`, `-prod`.

Pause here if you're doing a guided walkthrough.

### Stage 1.4: Request #1 — Flip the Feature Flag (mutates ConfigHub)

Ask: "This will change reservationMode in prod from strict to optimistic. OK?"

This stage uses the teaching-era ConfigHub `set-env` mutation command so you
can see the audit trail in ConfigHub. In the maintained `cub-gen` product path,
use `cub-gen springboot set-embedded-config` or
`./examples/springboot-paas/demo-embedded-config-mutation.sh` for the direct
embedded `application.yaml` apply-here proof.

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "release-day: reservation mode strict → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
```

Then show the evidence:

```bash
./confighub-compare.sh
```

You'll see `optimistic*` on prod — the `*` means it diverges from the upstream default.

```bash
./confighub-refresh-preview.sh prod
```

You'll see `PRESERVE` for the mutated field — your change survives when the generator re-renders.

```bash
cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'
```

You'll see the audit trail: who, when, and why.

Pause here if you're doing a guided walkthrough.

### Stage 1.5: Request #2 — Redis Caching (read-only)

```bash
./generator/render.sh --explain-field spring.cache.type
./lift-upstream.sh --explain
./lift-upstream.sh --render-diff
```

You'll see the field routes to `lift-upstream`, and a concrete diff bundle showing exactly what would change in `pom.xml` and `application.yaml`.

Explain: this change can't just be a ConfigHub mutation — it requires a new Maven dependency and Spring config changes. The bundle shows the exact patch, but automated PR creation is not implemented yet.

Pause here if you're doing a guided walkthrough.

### Stage 1.6: Request #3 — Datasource Override (read-only)

```bash
./generator/render.sh --explain-field spring.datasource.url
./block-escalate.sh --explain
./block-escalate.sh --render-attempt
```

You'll see the field is `generator-owned` by `platform-engineering`. The dry-run shows what the override would look like and why it should be blocked.

Explain: the datasource is provisioned by the platform. Letting app teams override it would bypass managed credentials and failover config. Server-side enforcement is not yet implemented.

Pause here if you're doing a guided walkthrough.

### Stage 1.7: Cleanup

```bash
./confighub-cleanup.sh
```

## Phase 2: One App Across Environments (springboot-platform-app-centric)

### Stage 2.1: The ADT View (read-only)

```bash
cd ../springboot-platform-app-centric
./setup.sh --explain
cat deployment-map.json | jq
```

You'll see `inventory-api` mapped to three deployments (dev/stage/prod), each becoming a ConfigHub space. The three mutation outcomes are listed.

Pause here if you're doing a guided walkthrough.

### Stage 2.2: Create and Demo (mutates ConfigHub)

Ask: "This will create spaces, units, and noop targets. OK?"

```bash
./setup.sh
./demo.sh
```

You'll see all three mutation outcomes walked through: apply-here succeeds, lift-upstream produces a bundle, block-escalate shows the boundary.

Pause here if you're doing a guided walkthrough.

### Stage 2.3: Cleanup

```bash
./cleanup.sh
```

## Phase 3: Platform Governing Multiple Apps (springboot-platform-platform-centric)

### Stage 3.1: The Platform View (read-only)

```bash
cd ../springboot-platform-platform-centric
./setup.sh --explain
./platform.sh --summary
./platform.sh --apps
```

You'll see one platform providing managed-datasource, runtime-hardening, and observability to two apps: `inventory-api` (3 deployments) and `catalog-api` (2 deployments).

Pause here if you're doing a guided walkthrough.

### Stage 3.2: Platform-Wide Field Ownership (read-only)

```bash
./platform.sh --explain-field spring.datasource.url
./platform.sh --explain-field feature.inventory.reservationMode
```

You'll see the datasource is BLOCKED for all apps on this platform, while the feature flag is MUTABLE because it's app-owned.

Pause here if you're doing a guided walkthrough.

### Stage 3.3: Create and Mutate (mutates ConfigHub)

Ask: "This will create 6 spaces and 5 units. OK?"

```bash
./setup.sh
```

Then mutate on both apps:

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "release-day: enable optimistic reservation mode" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic

cub function do --space catalog-api-prod --unit catalog-api \
  --change-desc "release-day: enable recommendations" \
  set-env catalog-api FEATURE_CATALOG_RECOMMENDATIONSENABLED=true
```

Both succeed — they're app-owned fields. In this teaching repo, a
platform-owned field is documented and previewed as a block/escalate boundary;
full server-side block/escalate enforcement is not implemented here.

Pause here if you're doing a guided walkthrough.

### Stage 3.4: Cleanup

```bash
./cleanup.sh
```

## What The User Should Understand

By the end of this challenge:

- How Spring config + platform policy become the Deployment, ConfigMap, and Service for the app
- Why some fields are mutable, some route back to source, some are blocked
- How the same model is visible as generator, app, or platform
- What is proven today and what is still incomplete
