# AI Start Here: frontend-postgres

## CRITICAL: Demo Pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen
3. Explain what the output means in plain English
4. If there is a GUI URL, print it
5. STOP and ask "Ready to continue?"
6. Only proceed when the human says to continue

## Suggested Prompt

```text
Read incubator/global-app-layer/frontend-postgres/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Example Teaches

This is a two-component layered recipe: `frontend` + `postgres` with a `backend-stub`. After the demo, the human will understand:

- Multi-component recipes in ConfigHub
- Deploy-time stubs as ConfigHub units
- Layered variant chains for small apps

## Prerequisites

- `cub` in PATH
- `jq` for JSON preview
- Authenticated ConfigHub CLI context for mutating steps
- Optional: live target for delivery proof

---

## Stage 1: "Check Capabilities" (read-only)

Run:

```bash
cd incubator/global-app-layer/frontend-postgres
which cub
cub version
cub context list --json | jq
cub target list --space "*" --json | jq '.[] | {space: .Space.Slug, target: .Target.Slug}'
```

What to explain:

- If `cub` is missing or auth fails, stay in preview mode
- If auth works but no target exists, use ConfigHub-only mode

GUI now: No GUI checkpoint for this stage — this is CLI-only.

GUI gap: No dashboard showing auth status and targets at a glance.

GUI feature ask: Auth status widget on landing page. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Preview The Recipe" (read-only)

Run:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Five spaces will be created (base → region → role → recipe → deploy)
- Two layered chains (frontend, postgres) plus one deploy-time backend-stub
- Nothing mutates yet

GUI now: No GUI checkpoint for this stage.

GUI gap: No visual recipe preview before materialization.

GUI feature ask: "Preview Recipe" button that shows planned spaces/units. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Materialize In ConfigHub" (mutates ConfigHub)

Ask: "This will create 5 spaces, multiple units, and the recipe manifest. Ready to proceed?"

Run:

```bash
./setup.sh
```

What to explain:

- Spaces and units are now in ConfigHub
- The printed GUI URLs are clickable
- Output goes to `.logs/setup.latest.log`

GUI now: Open the printed URLs. You should see:
- Recipe space with `recipe-us-staging-app` unit
- Deploy space with `frontend-cluster-a`, `postgres-cluster-a`, `backend-stub-cluster-a` units

GUI gap: No visual diff between "before setup" and "after setup".

GUI feature ask: Space creation wizard with before/after comparison. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 4: "Verify The Structure" (read-only)

Run:

```bash
./verify.sh
```

What to explain:

- Verifies all spaces, units, and links exist
- Verifies the recipe manifest contains correct provenance
- Output goes to `.logs/verify.latest.log`

GUI now: Compare verify output with GUI view of the units.

GUI gap: No automated verification badge on units.

GUI feature ask: Green checkmark on units passing verification. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 5: "Optional: Bind A Live Target" (mutates ConfigHub)

Only proceed if the human wants the live path AND a suitable target exists.

Run:

```bash
cd ..
./preflight-live.sh <space/target>
./preflight-live.sh <space/target> --json | jq

cd frontend-postgres
./set-target.sh <space/target>
```

What to explain:

- Preflight checks that the worker is ready
- `set-target.sh` binds deployment units to the target

GUI now: Inspect the unit and confirm target binding is visible.

GUI gap: No preflight status shown on target before binding.

GUI feature ask: Preflight check result on target card. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 6: "Optional: Apply Live" (mutates live infrastructure)

Only proceed if target is bound AND preflight passed.

Run:

```bash
source .state/state.env
cub unit approve --space "${PREFIX}-deploy-cluster-a" frontend-cluster-a
cub unit apply --space "${PREFIX}-deploy-cluster-a" frontend-cluster-a
```

What to explain:

- Approve makes the unit eligible for apply
- Apply sends rendered config to the target

GUI now: Inspect the unit after apply.

GUI gap: No live status badge showing apply success/failure.

GUI feature ask: Apply status with timestamp on unit card. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 7: "Cleanup"

Run:

```bash
./cleanup.sh
```

This removes all spaces and units created by the demo.

---

## What Mutates What

| Command | Writes |
|---------|--------|
| `./setup.sh --explain-json` | Nothing |
| `./setup.sh` | ConfigHub spaces, units, links, recipe manifest, local `.state/`, local `.logs/` |
| `./verify.sh` | local `.logs/verify.latest.log` |
| `./set-target.sh` | ConfigHub target bindings, local `.logs/set-target.latest.log` |
| `cub unit apply` | Live target state |

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [prompts.md](./prompts.md)
- [../whole-journey.md](../whole-journey.md)
