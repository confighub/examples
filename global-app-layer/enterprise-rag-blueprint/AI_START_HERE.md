# AI Start Here: enterprise-rag-blueprint

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
Read global-app-layer/enterprise-rag-blueprint/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Example Teaches

This is the app-layer counterpart to `gpu-eks-h100-training/`: NVIDIA's Enterprise RAG Blueprint expressed as a layered ConfigHub recipe with three deployment variants and five compliance initiatives. After the demo, the human will understand:

- The layered Blueprint shape: base → platform → accelerator → profile → recipe → deployment
- Four components moving together through the chain (rag-server, nim-llm, nim-embedding, vector-db)
- Three deployment variants (direct, Flux, Argo) — same as the substrate rung
- Five initiatives that turn blueprint compliance into kanban work
- The STACK selector that makes the same recipe work for stub, Ollama, or real NIM

## Prerequisites

- `cub` in PATH and authenticated
- `jq` for JSON preview and parsing

---

## Stage 1: "Check Capabilities" (read-only)

Run:

```bash
cd global-app-layer/enterprise-rag-blueprint
which cub jq
cub version
cub context list --json | jq
```

What to explain:

- If `cub` auth fails, stay in preview mode (only `--explain` will work).

GUI now: No GUI checkpoint for this stage.

**PAUSE.** Wait for the human.

---

## Stage 2: "Preview The Recipe" (read-only)

Run:

```bash
STACK=ollama ./setup.sh --explain
STACK=ollama ./setup.sh --explain-json | jq '{example, stack, prefix, namespace, spaces: (.spaces | length), components: ([.components[].component])}'
```

What to explain:

- Eight spaces will be created (base → platform → accelerator → profile → recipe → 3 deploy spaces).
- Four component chains (rag-server, nim-llm, nim-embedding, vector-db) — same shape as `realistic-app/` plus one more component.
- Three deployment variants at the leaf: direct, flux, argo.
- The STACK selector chooses image refs at the profile layer.

**PAUSE.** Wait for the human.

---

## Stage 3: "Materialize In ConfigHub" (mutates ConfigHub)

Ask: "This will create 8 spaces, ~60 units, and three deployment variants. Ready to proceed?"

Run:

```bash
STACK=ollama ./setup.sh
```

What to explain:

- Spaces and units are now in ConfigHub.
- Each layer's mutations are visible in the unit data.
- The recipe-manifest unit (`recipe-enterprise-rag-stack`) records the full chain with revisions.
- Three deployment variants exist; binding to a target is a separate explicit step.

GUI now: Open the printed Recipe space URL and the Direct deploy space URL.

**PAUSE.** Wait for the human.

---

## Stage 4: "Verify The Structure" (read-only)

Run:

```bash
./verify.sh
./verify.sh --json | jq '{ok, prefix, stack, spacesChecked: (.spacesChecked|length), unitsChecked: (.unitsChecked|length)}'
```

What to explain:

- Verifies all 8 spaces, all 61 units, all clone links.
- Verifies stack-specific assertions (e.g. for STACK=ollama, the deployment-layer rag-server unit must contain `host.docker.internal`).
- Verifies the recipe manifest contains all 4 components and all 3 variants.

**PAUSE.** Wait for the human.

---

## Stage 5: "Layer Initiatives Over The Chain" (mutates ConfigHub)

Ask: "This creates 5 Views in the recipe space, each filtering units of this run. Ready?"

Run:

```bash
./seed-initiatives.sh
```

What to explain:

- Five initiatives now exist as ConfigHub Views with Filters and metadata.
- They span pinned-model-versions, embed/index-dim match, GPU resource limits, guardrail policy, and resource limits.
- They use the same View+Filter+Trigger shape as `../../initiatives-demo/`. No `vet-kyverno` triggers are attached yet. For live policy enforcement, run `vet-kyverno` as a function bot on a ConfigHub Channel, as `../../initiatives-demo/` does.
- The script prints a View Explorer URL per initiative (`/x/view-explorer?view=<ViewID>`). The filters span all 8 spaces, so a single initiative covers direct + Flux + Argo variants.

GUI now: Open one of the printed View Explorer URLs (e.g. GPU Resource Limits) — you should see 6 units (2 GPU components × 3 deploy variants).

**PAUSE.** Wait for the human.

---

## Stage 6: "Optional: Show Upgrade Propagation" (mutates ConfigHub)

Run:

```bash
./upgrade-chain.sh llama3.2:1b nomic-embed-text
./verify.sh
```

What to explain:

- The bump happened at the profile layer.
- `cub unit push-upgrade` propagates the change down to recipe and deployment variants.
- Tenant-local values (namespace, region, the `LLM_HOST` override for STACK=ollama) survive the propagation. That's Story 1 (safe upgrades) made concrete.

**PAUSE.** Wait for the human.

---

## Stage 7: "Cleanup"

Run:

```bash
./cleanup.sh
```

This removes every space and unit.

---

## What Mutates What

| Command | Writes |
|---|---|
| `./setup.sh --explain` / `--explain-json` | Nothing |
| `./setup.sh` | ConfigHub spaces + units + clone links + recipe-manifest unit + local `.state/` + `.logs/` |
| `./verify.sh` | local `.logs/verify.latest.log` |
| `./seed-initiatives.sh` | ConfigHub Filters + Views in the recipe space |
| `./set-target.sh` | ConfigHub target bindings + re-rendered recipe manifest |
| `./upgrade-chain.sh` | ConfigHub unit data at profile/recipe/deployment layers |

## Related

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [prompts.md](./prompts.md)
- [../02-nvidia-blueprints-fit.md](../02-nvidia-blueprints-fit.md)
- [../whole-journey.md](../whole-journey.md)
