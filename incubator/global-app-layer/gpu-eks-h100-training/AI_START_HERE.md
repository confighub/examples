# AI Start Here: gpu-eks-h100-training

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
Read incubator/global-app-layer/gpu-eks-h100-training/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## Fastest Proven Live Lane

If the human explicitly wants the known-good local Flux OCI proof path, prefer:

```bash
./demo-flux-oci.sh --cleanup-first --target demo-flux/flux-renderer-worker-fluxoci-kubernetes-yaml-cluster
```

What to explain before running it:

- this mutates ConfigHub and the dedicated `demo-flux` kind cluster
- it auto-picks a safe short prefix because the Flux live path has a Kubernetes label budget
- it cleans old local Flux bridge objects on repeat runs
- it waits for Flux `Kustomization Ready=True` before closeout
- it is still structural proof with stub images, not functional GPU proof

## What This Example Teaches

This is an NVIDIA AICR-shaped layered recipe: `gpu-operator` + `nvidia-device-plugin` with three deployment variants. After the demo, the human will understand:

- NVIDIA's actual layering model: base → platform → accelerator → OS → recipe → deploy
- Multiple deployment variants at the leaf (direct, Flux, Argo)
- Structural proof with stub images (swap for real NVIDIA images for functional deployment)

## Prerequisites

- `cub` in PATH
- `jq` for JSON preview
- Authenticated ConfigHub CLI context for mutating steps
- Optional: live targets (Kubernetes for direct, FluxOCI for Flux, ArgoOCI for Argo)
- Optional: GPU-capable nodes and real images for functional NVIDIA proof

---

## Stage 1: "Check Capabilities" (read-only)

Run:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
which cub
cub version
cub context list --json | jq
cub target list --space "*" --json | jq '.[] | {space: .Space.Slug, target: .Target.Slug, provider: .Target.ProviderType}'
```

What to explain:

- If `cub` is missing or auth fails, stay in preview mode
- Note target provider types for routing to deployment variants:
  - `Kubernetes` → direct variant
  - `FluxOCI` or `FluxOCIWriter` → Flux variant
  - `ArgoOCI` → Argo variant
- `ArgoCDRenderer` and `FluxRenderer` are NOT deployment targets for this example

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

- Eight spaces will be created (base → platform → accelerator → OS → recipe → direct deploy → Flux deploy → Argo deploy)
- Two component chains (gpu-operator, nvidia-device-plugin)
- Three deployment variants at the leaf: direct, flux, argo
- Images are stubbed (`nginx:1.27-alpine`, `busybox:1.37`) for structural proof

GUI now: No GUI checkpoint for this stage.

GUI gap: No visual recipe preview before materialization.

GUI feature ask: "Preview Recipe" button that shows planned spaces/units. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Materialize In ConfigHub" (mutates ConfigHub)

Ask: "This will create 8 spaces, multiple units, and three deployment variants. Ready to proceed?"

Run:

```bash
./setup.sh
```

What to explain:

- Spaces and units are now in ConfigHub
- The printed GUI URLs are clickable
- Three deployment variants exist at the leaf
- Output goes to `.logs/setup.latest.log`

GUI now: Open the printed URLs. You should see:
- Recipe space with `recipe-eks-h100-ubuntu-training-stack` unit
- Three deploy spaces: direct, flux, argo variants

GUI gap: No visual diff between "before setup" and "after setup".

GUI feature ask: Space creation wizard with before/after comparison. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 4: "Verify The Structure" (read-only)

Run:

```bash
./verify.sh
./verify.sh --json | jq
```

What to explain:

- Verifies all spaces, units, and links exist
- Verifies the recipe manifest contains correct provenance
- Verifies all three deployment variants exist
- `./verify.sh --json` is the machine-readable verification seam
- Output goes to `.logs/verify.latest.log`

GUI now: Open each deploy space and compare the units.

GUI gap: No unified view showing all deployment variants for one recipe.

GUI feature ask: Deployment variant matrix view showing direct/flux/argo side by side. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 5: "Optional: Preflight Live Readiness" (read-only)

Only proceed if the human wants the live path.

Run:

```bash
cd ..
./preflight-live.sh <space/target>
./preflight-live.sh <space/target> --json | jq
```

What to explain:

- Target visibility is not the same as apply readiness
- Only proceed if `applyReady: true`
- The helper routes targets by provider type:
  - `Kubernetes` → direct deployment variant
  - `FluxOCI` / `FluxOCIWriter` → Flux deployment variant
  - `ArgoOCI` → Argo deployment variant

GUI now: No GUI checkpoint for this stage.

GUI gap: No preflight status shown on target card.

GUI feature ask: Preflight check result on target card before binding. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 6: "Optional: Bind Live Targets" (mutates ConfigHub)

Only proceed if preflight passed.

Run:

```bash
cd gpu-eks-h100-training
./set-target.sh <kubernetes-target>
./set-target.sh <fluxoci-target>
./set-target.sh <argooci-target>
```

What to explain:

- Each target is routed to the correct deployment variant by provider type
- You can bind one, two, or all three variants

GUI now: Inspect the matching deployment variant and confirm target binding.

GUI gap: No deployment variant matrix with target binding status.

GUI feature ask: Deployment variant matrix with binding and readiness status. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 7: "Optional: Apply Live" (mutates live infrastructure)

Only proceed if targets are bound.

Run:

```bash
source .state/state.env
# For direct variant:
cub unit approve --space "${PREFIX}-deploy-cluster-a" gpu-operator-cluster-a
cub unit apply --space "${PREFIX}-deploy-cluster-a" gpu-operator-cluster-a
```

What to explain:

- Approve makes the unit eligible for apply
- Apply sends rendered config to the target
- For Flux OCI: worker publishes to ConfigHub-native OCI origin, Flux reconciles
- For Argo OCI: worker publishes to ConfigHub-native OCI origin, ArgoCD reconciles

GUI now: Inspect the unit after apply.

GUI gap: No live status badge showing apply success/failure.

GUI feature ask: Apply status with timestamp on unit card. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 8: "Cleanup"

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
| `./set-target.sh` | ConfigHub target bindings for compatible variants, local `.logs/set-target.latest.log` |
| `cub unit apply` | Live target state |

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [prompts.md](./prompts.md)
- [../whole-journey.md](../whole-journey.md)
