# AI Start Here: single-component

Use this page when you want to drive `single-component` safely with an AI assistant.

## CRITICAL: Demo Pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen. If it is long, keep the important section visible and do not replace it with a one-line summary
3. Explain what the output means in plain English
4. If there is a GUI URL, print it: "You can also see this in the GUI: [URL]"
5. STOP and ask "Ready to continue?" or "Want to explore this more?"
6. Only proceed when the human says to continue

This is a demo, not a script execution. The value is in understanding each step.

## Suggested Prompt

```text
Read incubator/global-app-layer/single-component/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Don't move on until I say continue.
```

## What This Example Teaches

This is the smallest layered example in `global-app-layer`. After the demo, the human will understand:

- How ConfigHub models one layered variant chain (base → region → role → recipe → deploy)
- How the recipe manifest records provenance for the full chain
- How deployment variants work (direct, Flux OCI, Argo OCI)
- The difference between ConfigHub-only materialization and live delivery

## Prerequisites

- `cub` in PATH
- `jq` for JSON preview
- Authenticated ConfigHub CLI context for mutating steps
- Optional: live target for delivery proof

---

## Stage 1: "Check Capabilities" (read-only)

Run:

```bash
cd incubator/global-app-layer/single-component
which cub
cub version
cub context list --json | jq
cub target list --space "*" --json | jq '.[] | {space: .Space.Slug, target: .Target.Slug, providerType: .Target.ProviderType}'
```

What to explain:

- If `cub` is missing or auth fails, stop at preview mode
- If auth works but no target is available, use ConfigHub-only mode
- Note which targets exist (Kubernetes, FluxOCI, ArgoCDOCI)

GUI now: None yet; this is CLI-only capability check.

GUI gap: No dashboard showing auth status and available targets at a glance.

GUI ask: Auth status widget on landing page showing context + target count.

**PAUSE.** Wait for the human.

---

## Stage 2: "Preview The Layered Recipe" (read-only)

Run:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- The 7 spaces that will be created (4 catalog + 1 recipe + 3 deploy variants)
- The variant chain: base → us → us-staging → recipe → deploy leaves
- The three deployment variants: direct, flux, argo
- Nothing mutates yet

GUI now: None; this is a preview of what will be created.

GUI gap: No visual recipe preview before materialization.

GUI ask: "Preview Recipe" button that shows the planned spaces/units before creation.

**PAUSE.** Wait for the human.

---

## Stage 3: "Materialize In ConfigHub" (mutates ConfigHub)

Ask: "This will create 7 spaces and multiple units in ConfigHub. Ready to proceed?"

Run:

```bash
./setup.sh
```

What to explain:

- Spaces and units are now created in ConfigHub
- The GUI URLs printed at the end are clickable
- The `.logs/setup.latest.log` file captures the full output
- No live deployment yet - this is ConfigHub-only

GUI now: Open the printed URLs. You should see:
- Recipe space with `backend-recipe-us-staging` unit
- Deploy space with `backend-cluster-a` unit
- Recipe manifest unit showing the full chain provenance

GUI gap: No visual diff between "before setup" and "after setup".

GUI ask: Space creation wizard showing before/after comparison.

**PAUSE.** Wait for the human.

---

## Stage 4: "Verify The Structure" (read-only)

Run:

```bash
./verify.sh
```

What to explain:

- Verifies all spaces, units, and links exist
- Verifies the recipe manifest unit contains correct provenance
- Verifies the deployment variants are correctly linked
- Output goes to `.logs/verify.latest.log`

GUI now: Compare the verify output with the GUI view of the units.

GUI gap: No automated verification status badge on units.

GUI ask: Green checkmark on units that pass structural verification.

**PAUSE.** Wait for the human.

---

## Stage 5: "Inspect The Deployment Variants" (read-only)

Run:

```bash
# Show the three deployment variants
cub unit list --space "$(cat .state/state.env | grep PREFIX | cut -d= -f2)-deploy-cluster-a" --json | jq '.[] | {unit: .Unit.Slug}'
cub unit list --space "$(cat .state/state.env | grep PREFIX | cut -d= -f2)-deploy-cluster-a-flux" --json | jq '.[] | {unit: .Unit.Slug}'
cub unit list --space "$(cat .state/state.env | grep PREFIX | cut -d= -f2)-deploy-cluster-a-argo" --json | jq '.[] | {unit: .Unit.Slug}'
```

What to explain:

- Three deployment variants exist at the leaf: direct, flux, argo
- Each variant shares the same upstream recipe unit
- The delivery mode is determined by target type, not unit content

GUI now: Open each deploy space in ConfigHub and compare the units.

GUI gap: No unified view showing all deployment variants for one recipe.

GUI ask: Deployment variant matrix view showing direct/flux/argo side by side.

**PAUSE.** Wait for the human.

---

## Stage 6: "Optional: Bind A Live Target" (mutates ConfigHub)

Only proceed if the human wants the live path AND a suitable target exists.

Run:

```bash
# Check preflight first
cd ..
./preflight-live.sh <space/target>
./preflight-live.sh <space/target> --json | jq

# If ready, bind the target
cd single-component
./set-target.sh <space/target>
```

What to explain:

- Target visibility is not the same as apply readiness
- Preflight checks that the worker is actually ready
- `set-target.sh` binds the deployment unit to the target

GUI now: Inspect the unit and confirm target binding is visible.

GUI gap: No preflight status shown on target before binding.

GUI ask: Preflight check result shown on target card before any binding.

**PAUSE.** Wait for the human.

---

## Stage 7: "Optional: Apply Live" (mutates live infrastructure)

Only proceed if target is bound AND preflight passed.

Run:

```bash
source .state/state.env
cub unit approve --space "${PREFIX}-deploy-cluster-a" backend-cluster-a
cub unit apply --space "${PREFIX}-deploy-cluster-a" backend-cluster-a
```

What to explain:

- Approve makes the unit eligible for apply
- Apply sends the rendered config to the target
- For Direct Kubernetes: worker applies via kubectl
- For Flux OCI: worker publishes to ConfigHub-native OCI, Flux reconciles
- For Argo OCI: worker publishes to ConfigHub-native OCI, Argo reconciles

GUI now: Inspect the unit after apply. Compare intended state vs live result.

GUI gap: No live status badge showing apply success/failure.

GUI ask: Apply status with timestamp and evidence link on unit card.

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
| `./set-target.sh` | ConfigHub target bindings, local `.logs/set-target.latest.log` |
| `cub unit apply` | Live target state |

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [prompts.md](./prompts.md)
- [../whole-journey.md](../whole-journey.md) for the full lifecycle
