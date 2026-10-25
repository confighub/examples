# AI Start Here: Spring Boot Platform App (ADT View)

Read [`README.md`](./README.md) first. This page is for running the example yourself or pairing with an AI assistant.

## CRITICAL: Demo Pacing

When walking a human through this example, pause after every stage.

After each stage:
1. run the commands for that stage
2. show the important output
3. explain what the output means
4. say what the output proves
5. if there is a GUI checkpoint, print it
6. stop and wait for the human before continuing

## Fast Preview

```bash
cub context list --json
cub space list --json

cd spring-platform/springboot-platform-app-centric
./setup.sh --explain
./setup.sh --explain-json | jq
./demo.sh
```

If `cub space list --json` fails because auth is missing or expired, run `cub auth login` before any mutating stage.

This path is read-only. It is useful for orientation, but it does **not** prove that setup or mutation actually work.

## Fast Operational Evaluation

If you want a quick but meaningful proof that the example actually works, run:

```bash
cub context list --json
cub space list --json

cd spring-platform/springboot-platform-app-centric
./setup.sh
cub space list --where "Labels.ExampleName = 'springboot-platform-app-centric'" --json | jq '.[].Space.Slug'
./verify.sh
./demo.sh

cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "demo: reservation mode strict -> optimistic" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic

cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '.[] | {mutationNum: .Mutation.MutationNum, source: .Revision.Source, description: .Revision.Description, createdAt: .Mutation.CreatedAt, author: .Author.Email}'
```

Then stop before `./cleanup.sh` unless the human asks for cleanup.

This proves:
- setup succeeds
- spaces are isolated by `ExampleName`
- the three mutation outcomes are still explained with stable CLI output
- one real `apply-here` mutation works
- mutation history is captured

## Pairing Modes

- Solo evaluation: run a full phase, then summarize what mattered.
- Guided walkthrough: run one stage at a time and pause at the marked checkpoints.

## Suggested Prompts

```text
Read spring-platform/springboot-platform-app-centric/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Do not continue until I say continue.
```

```text
Read spring-platform/springboot-platform-app-centric/AI_START_HERE.md and help me evaluate it quickly.
Use the fast preview first, then continue into the fast operational evaluation unless I explicitly ask for read-only mode. Summarize what matters after each phase, and only pause when I ask.
```

For a repeat-until-boring pass, use:

- [`AI_REPEAT_RUN_PROMPT.md`](./AI_REPEAT_RUN_PROMPT.md)

## What This Example Teaches

After this demo, the human will understand:
- The App-Deployment-Target model
- How ConfigHub spaces represent deployments
- The three mutation outcomes (apply-here, lift-upstream, block-escalate)
- How to run mutations with full audit trail

No cluster required. Units live in ConfigHub only.

## Stage 1: "What Is This App?" (read-only)

```bash
cd spring-platform/springboot-platform-app-centric
cat deployment-map.json | jq
```

You'll see:
- `inventory-api` is the app
- Three deployments: dev, stage, prod
- Each deployment becomes a ConfigHub space

GUI now: No GUI checkpoint for this stage — this is CLI-only orientation.

GUI gap: No visual map of app → deployment → target relationships.

GUI feature ask: App deployment topology view. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 2: "Preview Setup" (read-only)

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
```

You'll see:
- ASCII diagram shows the App → Deployments → Targets structure
- Three mutation outcomes are listed
- This is read-only — no ConfigHub changes yet

GUI now: No GUI checkpoint for this stage — preview is CLI-only.

GUI gap: No web-based preview of setup plan.

GUI feature ask: Setup preview wizard. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 3: "Create The Config" (mutates ConfigHub)

```bash
./setup.sh
cub space list --where "Labels.ExampleName = 'springboot-platform-app-centric'" --json | jq '.[].Space.Slug'
```

You'll see:
- 3 spaces created (dev, stage, prod)
- Each space has one unit

GUI now: ConfigHub → Spaces → filter `ExampleName=springboot-platform-app-centric`

GUI gap: No way to see deployment hierarchy at a glance.

GUI feature ask: Space grouping by app. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 4: "Three Mutation Outcomes" (read-only)

```bash
./demo.sh
```

You'll see:
- **apply-here**: changes apply directly (e.g., feature flags)
- **lift-upstream**: changes require upstream input modification (e.g., database)
- **block-escalate**: changes are blocked by platform policy (e.g., secrets)

GUI now: No GUI checkpoint — this stage explains concepts via CLI output.

GUI gap: No visual mutation route badges on fields.

GUI feature ask: Color-coded field ownership indicators. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 5: "Try A Mutation" (mutates ConfigHub)

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "demo: reservation mode strict → optimistic" \
  set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic

cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '.[] | {mutationNum: .Mutation.MutationNum, source: .Revision.Source, description: .Revision.Description, createdAt: .Mutation.CreatedAt, author: .Author.Email}'
```

You'll see:
- The mutation is stored with full audit trail
- Description and mutation order are captured
- Timestamp and actor fields may vary by surface, so report the live output honestly
- This proves the ConfigHub mutation plane works

GUI now: Open unit → History tab → see the mutation with author and description.

GUI gap: No diff view showing exactly what changed.

GUI feature ask: Inline mutation diff viewer. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 6: "Cleanup"

```bash
./cleanup.sh
```

This removes all spaces and units created by this demo.

## What Mutates What

| Command | Writes |
|---------|--------|
| `./setup.sh --explain` | Nothing |
| `./setup.sh --explain-json` | Nothing |
| `./setup.sh` | ConfigHub spaces and units |
| `cub function do` | Unit mutation |
| `./cleanup.sh` | Deletes ConfigHub objects |

## Not Yet Implemented

- `lift upstream` automated PR
- `block/escalate` server-side enforcement

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
