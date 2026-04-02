# AI Start Here: Spring Boot Generator Example

Read [`README.md`](./README.md) first. This page is for running the example yourself or pairing with an AI assistant.

## Fast Path

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

If `cub space list --json` fails because auth is missing or expired, run `cub auth login` before any mutating stage.

## Pairing Modes

- Solo evaluation: run a full phase, then summarize what mattered.
- Guided walkthrough: run one stage at a time and pause at the marked checkpoints.

## Suggested Prompts

```text
Read spring-platform/springboot-platform-app/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Do not continue until I say continue.
```

```text
Read spring-platform/springboot-platform-app/AI_START_HERE.md and help me evaluate it quickly.
Use the fast path first, summarize what matters after each phase, and only pause when I ask.
```

## What This Example Teaches

After this demo, the human will understand:
- The generator model (inputs + policies → operational config)
- Field-level lineage tracing
- The three mutation routes: apply-here, lift-upstream, block-escalate
- How field ownership determines what changes are allowed

No cluster required. Uses ConfigHub-only mode by default.

## Stage 1: "Preview" (read-only)

```bash
cd spring-platform/springboot-platform-app
./setup.sh --explain
./verify.sh
```

You'll see:
- Shows the generator model structure
- Lists input/output paths
- Describes the three mutation routes
- Verify confirms local fixtures are in place

GUI now: No GUI checkpoint for this stage — preview is CLI-only.

GUI gap: No web-based generator diagram.

GUI feature ask: Generator lineage visualization. No issue filed yet.

Pause here if you're doing a guided walkthrough.

## Stage 2: "Generator Transformation" (read-only)

```bash
./generator/render.sh --explain
./generator/render.sh --trace
```

You'll see:
- App inputs + platform policies combine to produce operational config
- The trace shows field-by-field transformation
- Each field has a known origin

Then show field lineage:

```bash
./generator/render.sh --explain-field spring.datasource.url
./generator/render.sh --explain-field feature.inventory.reservationMode
```

You'll see:
- `spring.datasource.url` is BLOCKED (platform-injected)
- `feature.inventory.reservationMode` is MUTABLE (app-owned)

GUI now: No GUI checkpoint — lineage is CLI-only for now.

GUI gap: No field ownership badges in the GUI.

GUI feature ask: Color-coded field ownership in unit viewer. No issue filed yet.

Pause here if you're doing a guided walkthrough.

## Stage 3: "ConfigHub Setup" (mutates ConfigHub)

```bash
./confighub-setup.sh --explain
./confighub-setup.sh
./confighub-verify.sh
```

You'll see:
- 3 spaces created (dev, stage, prod)
- Each has one unit (`inventory-api`)
- Verify confirms the expected structure

GUI now: ConfigHub → Spaces → filter `ExampleName=springboot-platform-app`

GUI gap: No visual generator lineage from GUI.

GUI feature ask: Upstream inputs visible from unit view. No issue filed yet.

Pause here if you're doing a guided walkthrough.

## Stage 4: "Apply-Here Mutation" (mutates ConfigHub)

```bash
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: reservation mode strict → optimistic" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

cub mutation list --space inventory-api-prod --json inventory-api | \
  jq '[.[-1] | {mutationNum, description, author: .Author.Email, createdAt: .CreatedAt}]'
```

You'll see:
- The mutation is stored with full audit trail
- Apply-here works because the field is app-owned

GUI now: Open unit → History tab → see the mutation.

GUI gap: No indication that this field was apply-here vs another route.

GUI feature ask: Route badge on mutation history entries. No issue filed yet.

Pause here if you're doing a guided walkthrough.

## Stage 5: "Lift-Upstream Bundle" (read-only)

```bash
./generator/render.sh --explain-field spring.cache.type
./lift-upstream.sh --explain
./lift-upstream.sh --render-diff
```

You'll see:
- Shows the Redis cache bundle
- Demonstrates what upstream changes would be needed
- The diff shows exact input modifications required

GUI now: No GUI checkpoint — lift-upstream is CLI-only.

GUI gap: No lift-upstream proposal workflow in GUI.

GUI feature ask: Upstream change proposal wizard. No issue filed yet.

Pause here if you're doing a guided walkthrough.

## Stage 6: "Block/Escalate Boundary" (read-only)

```bash
./generator/render.sh --explain-field spring.datasource.url
./block-escalate.sh --explain
./block-escalate.sh --render-attempt
```

You'll see:
- Shows the boundary documentation
- Platform-owned fields cannot be mutated directly
- Server-side enforcement is not yet implemented

GUI now: No GUI checkpoint — boundary documentation is CLI-only.

GUI gap: No visual indication of blocked fields.

GUI feature ask: Red "blocked" badge on platform-owned fields. No issue filed yet.

Pause here if you're doing a guided walkthrough.

## Stage 7: "Cleanup"

```bash
./confighub-cleanup.sh
```

This removes all spaces and units created by this demo.

## Optional: Real Kubernetes

Requires Kind cluster:

```bash
./bin/create-cluster && ./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker
export KUBECONFIG=var/springboot-platform.kubeconfig WORKER_SPACE=springboot-infra
./confighub-setup.sh --with-targets
./verify-e2e.sh
./bin/teardown
```

## What Mutates What

| Command | Writes |
|---------|--------|
| `./setup.sh --explain` | Nothing |
| `./generator/render.sh --explain` | Nothing |
| `./confighub-setup.sh --explain` | Nothing |
| `./confighub-setup.sh` | ConfigHub spaces, units |
| `cub function do` | Unit mutation |
| `./confighub-cleanup.sh` | Deletes ConfigHub objects |

## Not Yet Implemented

- `lift upstream` automated PR
- `block/escalate` server-side enforcement

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
