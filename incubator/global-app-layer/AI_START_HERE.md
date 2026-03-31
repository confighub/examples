# AI Start Here: global-app-layer

## CRITICAL: Demo Pacing

When walking a human through this package, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen
3. Explain what the output means in plain English
4. If there is a GUI URL, print it
5. STOP and ask "Ready to continue?"
6. Only proceed when the human says to continue

## Suggested Prompt

```text
Read incubator/global-app-layer/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Package Teaches

This package demonstrates layered recipes as real ConfigHub objects. After the demo, the human will understand:

- How ConfigHub models variant chains (base → region → role → recipe → deploy)
- How recipe manifests record provenance
- How deployment variants work (direct, Flux OCI, Argo OCI)
- The difference between ConfigHub-only materialization and live delivery

## Start With single-component

**The first wedge is [single-component](./single-component/AI_START_HERE.md).** Go there first.

Only use the other examples after single-component has created value.

---

## Stage 1: "Check Capabilities" (read-only)

Run:

```bash
cd incubator/global-app-layer
which cub
cub version
cub context list --json | jq
```

What to explain:

- If `cub` is missing or auth fails, stay in preview mode
- If auth works but no target exists, use ConfigHub-only mode

GUI now: No GUI checkpoint for this stage — this is CLI-only.

GUI gap: No dashboard showing auth status at a glance.

GUI feature ask: Auth status widget on landing page. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Discover Active Runs" (read-only)

Run:

```bash
./find-runs.sh
./find-runs.sh --json | jq
```

What to explain:

- Shows any existing global-app-layer runs in ConfigHub
- Helps avoid prefix collisions

GUI now: No GUI checkpoint for this stage.

GUI gap: No example-aware grouping in ConfigHub GUI.

GUI feature ask: Filter by `Labels.ExampleName` in space list. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Choose An Example" (read-only)

| Example | Best for |
|---------|----------|
| [single-component](./single-component/AI_START_HERE.md) | Smallest proof, start here |
| [frontend-postgres](./frontend-postgres/AI_START_HERE.md) | Two components |
| [realistic-app](./realistic-app/AI_START_HERE.md) | Three-component app |
| [gpu-eks-h100-training](./gpu-eks-h100-training/AI_START_HERE.md) | NVIDIA AICR-shaped stack |

Recommend `single-component` unless the human asks for something specific.

**PAUSE.** Ask "Which example should we run?"

---

## Stage 4: "Preview The Example" (read-only)

Run (using single-component as example):

```bash
cd single-component
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Shows what spaces, units, and links will be created
- Nothing is mutated yet

GUI now: No GUI checkpoint for this stage.

GUI gap: No visual recipe preview before materialization.

GUI feature ask: "Preview Recipe" button. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Continue With The Example

From here, switch to the example's own `AI_START_HERE.md`:

- [single-component/AI_START_HERE.md](./single-component/AI_START_HERE.md)
- [frontend-postgres/AI_START_HERE.md](./frontend-postgres/AI_START_HERE.md)
- [realistic-app/AI_START_HERE.md](./realistic-app/AI_START_HERE.md)
- [gpu-eks-h100-training/AI_START_HERE.md](./gpu-eks-h100-training/AI_START_HERE.md)

---

## Delivery Matrix

| Mode | Status |
|------|--------|
| **Direct Kubernetes** | Fully working |
| **Flux OCI** | Current standard |
| **Argo OCI** | Implemented |
| **ArgoCDRenderer** | Renderer path only |

## Preflight Before Live

Do not rely on target visibility alone. Before the live path:

```bash
./preflight-live.sh <space/target>
./preflight-live.sh <space/target> --json | jq
```

Only proceed if `applyReady: true`.

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [prompts.md](./prompts.md)
- [whole-journey.md](./whole-journey.md)
