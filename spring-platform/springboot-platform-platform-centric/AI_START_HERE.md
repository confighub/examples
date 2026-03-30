# AI Start Here: Spring Boot Platform (Experimental ADTP View)

This page helps AI assistants guide humans through this example.

Read the [`README.md`](./README.md) first. It explains the model. This page
explains how to demo it.

**Note:** ADTP is experimental. The model is sound but tooling is incomplete.

## Demo Pacing Rules

Pause after every stage:

1. Run only that stage's commands
2. Print the full output (do not summarize)
3. Explain what the output means
4. Print GUI checkpoints where applicable
5. Ask "Ready to continue?" and wait

## Suggested Prompt

```text
Read spring-platform/springboot-platform-platform-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Explain what is platform-owned versus app-owned.
Do not continue until I say continue.
```

## Stage 1: What Is This Platform? (read-only)

```bash
cd spring-platform/springboot-platform-platform-centric
cat platform-map.json | jq
```

What to explain:
- The platform is `springboot-platform`
- It provides managed datasource, hardening, observability
- Two apps: `inventory-api` (3 deployments), `catalog-api` (2 deployments)
- Platform controls `spring.datasource.*` and `securityContext.*`

GUI checkpoint: none

**PAUSE.**

## Stage 2: What Does The Platform Provide? (read-only)

```bash
./platform.sh --summary
```

What to explain:
- Managed datasource: PostgreSQL with HA, encryption, backups
- Runtime hardening: Security defaults
- Observability: Health endpoints, SLO targets
- All apps inherit these capabilities

GUI checkpoint: none

**PAUSE.**

## Stage 3: What Apps Run On This Platform? (read-only)

```bash
./platform.sh --apps
```

What to explain:
- `inventory-api` has 3 deployments (dev, stage, prod)
- `catalog-api` has 2 deployments (dev, prod)
- Each app has its own app-owned fields
- Both share platform-controlled fields

GUI checkpoint: none

**PAUSE.**

## Stage 4: What Will Setup Create? (read-only)

```bash
./setup.sh --explain
```

What to explain:
- 1 infra space (server worker)
- 5 app spaces (3 for inventory-api, 2 for catalog-api)
- 5 units, 5 noop targets
- Default mode is noop

GUI checkpoint: none

**PAUSE.**

## Stage 5: Create The Platform (mutates ConfigHub)

Ask: "This will create 6 spaces, 5 units, and 5 noop targets. OK?"

```bash
./setup.sh
```

Verify:

```bash
cub space list --where "Labels.Platform = 'springboot-platform'" --json | jq '.[].Space.Slug'
```

What to explain:
- Six spaces now exist in ConfigHub
- All tagged with the same platform label
- Platform team can see all apps with one query

GUI checkpoint: Open ConfigHub GUI → Spaces → filter by `Platform=springboot-platform`

**PAUSE.**

## Stage 6: Field Ownership (read-only)

Show why some fields are blocked platform-wide:

```bash
./platform.sh --explain-field spring.datasource.url
```

What to explain:
- This field is platform-owned
- Applies to ALL apps on this platform
- App teams cannot change it

Now show an app-owned field:

```bash
./platform.sh --explain-field feature.inventory.reservationMode
```

What to explain:
- This field is app-owned (inventory-api only)
- Safe to change directly in ConfigHub

GUI checkpoint: none

**PAUSE.**

## Stage 7: Mutate On Both Apps (mutates ConfigHub)

Ask: "This will change feature flags on both apps. OK?"

For inventory-api:

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "demo: enable optimistic reservation mode" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic
```

For catalog-api:

```bash
cub function do --space catalog-api-prod --unit catalog-api \
  --change-desc "demo: enable recommendations" \
  set-env catalog-api FEATURE_CATALOG_RECOMMENDATIONSENABLED=true
```

What to explain:
- Both mutations succeed because they're app-owned fields
- Platform-owned fields would be blocked
- Each app has its own mutation history

Show the mutation history:

```bash
cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'
```

GUI checkpoint: Open each unit → History → see the mutations

**PAUSE.**

## Stage 8: Running App (optional, requires Java)

Ask: "Do you want to see the running app confirm the mutation? This requires Java 21+ and Maven."

If yes:

```bash
cd ../springboot-platform-app/upstream/app
FEATURE_INVENTORY_RESERVATIONMODE=optimistic \
  mvn spring-boot:run -q -Dspring-boot.run.profiles=prod \
  -Dspring-boot.run.arguments="--server.port=8081" &
APP_PID=$!
sleep 10
curl -s http://localhost:8081/api/inventory/summary | jq
kill $APP_PID
cd ../../springboot-platform-platform-centric
```

What to explain:
- The app reports `optimistic`
- The mutation is visible in a real HTTP response

**PAUSE.**

## Stage 9: Cleanup

```bash
./cleanup.sh
```

## Key Insight

The experimental ADTP view answers: "I manage multiple apps — how do I organize shared policies?"

- Platform provides capabilities that all apps inherit
- Platform controls fields that should never diverge per-app
- Apps can still mutate their own feature flags
- One place to see all apps: `./platform.sh --apps`

## What This Does Not Prove

- `lift upstream` via automated GitHub PR
- `block/escalate` via server-side enforcement
- Flux/Argo delivery (see `global-app-layer` examples)
- Platform-as-resource in ConfigHub (experimental, label-based only)
