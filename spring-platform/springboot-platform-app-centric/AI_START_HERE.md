# AI Start Here: Spring Boot Platform App (ADT View)

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
Read spring-platform/springboot-platform-app-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## Stage 1: What Is This App? (read-only)

```bash
cd spring-platform/springboot-platform-app-centric
cat deployment-map.json | jq
```

What to explain:
- The app is `inventory-api`
- Three deployments: dev, stage, prod
- Each deployment becomes a ConfigHub space
- Three mutation outcomes: apply-here, lift-upstream, block-escalate

GUI checkpoint: none

**PAUSE.**

## Stage 2: What Will Setup Create? (read-only)

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:
- The ASCII diagram shows App → Deployments → Targets
- Default mode is noop: apply workflow works without a cluster
- The mutation outcomes table shows which fields route where

GUI checkpoint: none (preview only)

**PAUSE.**

## Stage 3: Field Ownership (read-only)

Show why some fields are mutable vs blocked:

```bash
../springboot-platform-app/generator/render.sh --explain-field spring.datasource.url
```

What to explain:
- This field is BLOCKED because the generator injects it from platform policy
- It's not in app inputs - it comes from `runtime-policy.yaml`

Now show a mutable field:

```bash
../springboot-platform-app/generator/render.sh --explain-field feature.inventory.reservationMode
```

What to explain:
- This field is MUTABLE because it comes from app inputs
- The generator passes it through without transformation

GUI checkpoint: none

**PAUSE.**

## Stage 4: Create The Config (mutates ConfigHub)

Ask: "This will create 4 spaces (3 env + 1 infra), 3 units, 3 noop targets, and apply all units. OK?"

```bash
./setup.sh
```

Verify:

```bash
cub space list --where "Labels.ExampleName = 'springboot-platform-app'" --json | jq '.[].Space.Slug'
```

What to explain:
- Four spaces now exist in ConfigHub
- Each env space has a unit and a noop target
- The infra space has a server worker

GUI checkpoint: Open ConfigHub GUI → Spaces → filter by `ExampleName=springboot-platform-app`

**PAUSE.**

## Stage 5: Three Mutation Outcomes (read-only)

```bash
./demo.sh
```

What to explain:
- **Apply here**: `feature.inventory.*` — change directly in ConfigHub
- **Lift upstream**: `spring.cache.*` — needs upstream source changes
- **Block/escalate**: `spring.datasource.*` — platform-owned

GUI checkpoint: none

**PAUSE.**

## Stage 6: Try A Mutation (mutates ConfigHub)

Ask: "This will change the reservation mode for prod. OK?"

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "demo: reservation mode strict → optimistic" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic
```

Show the mutation history:

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

## Stage 7: Apply To Target (mutates ConfigHub)

```bash
cub unit apply --space inventory-api-prod inventory-api
```

What to explain:
- The unit was applied to the noop target
- Noop accepts the apply but doesn't deliver to a real cluster
- Use `--with-targets` for real delivery

GUI checkpoint: Open unit → see sync status

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
cd ../../springboot-platform-app-centric
```

Expected output:

```json
{
  "service": "inventory-api",
  "environment": "prod",
  "reservationMode": "optimistic",
  "cacheBackend": "none"
}
```

What to explain:
- The app reports `optimistic`
- The mutation from Stage 6 is visible in a real HTTP response

**PAUSE.**

## Stage 9: Flow Docs (read-only)

```bash
cat flows/apply-here.md
```

What to explain:
- Detailed walkthrough of the apply-here mutation path
- Also available: `flows/lift-upstream.md`, `flows/block-escalate.md`

**PAUSE.**

## Stage 10: Cleanup

```bash
./cleanup.sh
```

## What This Does Not Prove

- `lift upstream` via automated GitHub PR
- `block/escalate` via server-side enforcement
- Flux/Argo delivery (see `global-app-layer` examples)
