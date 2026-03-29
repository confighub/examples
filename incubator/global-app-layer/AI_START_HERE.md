# AI Start Here

Use this page when you want to drive the `global-app-layer` package safely with Codex, Claude, Cursor, or another AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. ask `Ready to continue?`
6. wait for the human before proceeding

This package is easy to over-rush. Do not jump from preview to setup to target binding to apply in one burst.

## Suggested Prompt

```text
Read incubator/global-app-layer/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Package Is For

This package demonstrates how ConfigHub can represent layered recipes as real versioned config objects.

It is for:

- multi-component app stacks
- NVIDIA AICR-style layered recipes
- safe updates and downstream propagation
- optional direct or delegated delivery after the recipe is materialized

It is not only about live deployment. A large part of the value is visible in the ConfigHub database before anything is applied to a cluster.

## Delivery Matrix

Know which delivery mode is in scope before making claims:

| Delivery Mode | Status | Use case |
|---------------|--------|----------|
| **Direct Kubernetes** | Fully working | Simplest real proof. No controller required. |
| **Flux OCI** | Current standard | Worker publishes to ConfigHub-native OCI origin. Flux manages workload lifecycle. |
| **Argo OCI** | Implemented | Worker publishes to ConfigHub-native OCI origin. Argo reconciles workloads. |
| **ArgoCDRenderer** | Working, limited scope | Renderer path only. Expects Argo `Application` payloads. Not OCI delivery. |

Critical distinctions:

- **Flux OCI** is the current standard controller-oriented delivery path
- **ArgoCDRenderer** is **not** Argo OCI delivery — it is a renderer path for hydration only
- **Argo OCI** is now implemented in `single-component` and `gpu-eks-h100-training`, but only claim it when controller and live evidence are shown
- Raw-manifest examples work with Direct Kubernetes and Flux OCI broadly, and selected examples now also work with Argo OCI, but not with ArgoCDRenderer

## Bundle Boundary

If the question is specifically about AICR bundles, checksums, SBOMs, or attestations, be explicit:

- this package explains the bundle story honestly
- it includes a fixture-backed evidence sample
- Flux OCI is the current controller-oriented bundle path
- Argo OCI is now implemented in selected examples, but package-level bundle inspection is still incomplete
- it does not yet prove a fully real in-product bundle publication and inspection flow

Use these files with that boundary in mind:

- [05-bundle-publication-walkthrough.md](./05-bundle-publication-walkthrough.md)
- [04-bundles-attestation-and-todo.md](./04-bundles-attestation-and-todo.md)
- [bundle-evidence-sample/README.md](./bundle-evidence-sample/README.md)
- [06-bundle-evidence-gui-spec.md](./06-bundle-evidence-gui-spec.md)

## Choose The Smallest Matching Demo

Before you run this package, pick the smallest demo that matches the user's actual question:

| User goal | Better first stop |
|---|---|
| Show me Argo import from GitHub | [gitops-import-argo](../gitops-import-argo/README.md) |
| Show me Flux import from GitHub | [gitops-import-flux](../gitops-import-flux/README.md) |
| Show me Helm first | [helm-platform-components](../../helm-platform-components/README.md) |
| Show me the smallest direct apply example | [single-component](./single-component/README.md) |
| Show me App-Deployment-Target | [promotion-demo-data](../../promotion-demo-data/README.md) |
| Show me microservices or app-of-apps styles | [apptique-flux-monorepo](../apptique-flux-monorepo/README.md) |

Use `global-app-layer` when the question is specifically about:

- layered recipes
- deployment units at the leaf
- upstream propagation with preserved downstream specialization
- NVIDIA-shaped configuration chains

If the question is specifically about AICR bundles, checksums, SBOMs, or attestations, also read:

- [05-bundle-publication-walkthrough.md](./05-bundle-publication-walkthrough.md)
- [04-bundles-attestation-and-todo.md](./04-bundles-attestation-and-todo.md)
- [bundle-evidence-sample/README.md](./bundle-evidence-sample/README.md)
- [06-bundle-evidence-gui-spec.md](./06-bundle-evidence-gui-spec.md)

## WET-First, Not Live-First

This package is intended-state first.

The normal path is:

1. preview the layered recipe
2. materialize it in ConfigHub as WET objects
3. verify it in ConfigHub
4. optionally bind a target
5. optionally apply live

So `setup.sh` is ConfigHub-first, not cluster-first.

## Stage 1: Preview The Layered Recipe (read-only)

```bash
git rev-parse --show-toplevel
which cub
kubectl version --client 2>/dev/null || true
cub version
cub context list --json | jq

cd incubator/global-app-layer
./find-runs.sh --json | jq

cd realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

What these do not mutate:

- they do not create spaces
- they do not create units
- they do not bind targets
- they do not apply to a cluster

GUI now: None yet; this stage is preview only.

GUI gap: No visual recipe preview before materialization.

GUI ask: "Preview Recipe" button that shows planned spaces/units before creation.

**PAUSE.** Wait for the human.

## Stage 2: Materialize In ConfigHub (mutates ConfigHub)

ConfigHub-only path:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh
./verify.sh
```

What you should see after:

- recipe and deploy spaces created in ConfigHub
- deployment units and links materialized as WET objects
- durable logs in `.logs/`
- printed GUI URLs for the recipe space, deploy space, manifest, and one deployment unit

GUI now: Open the printed ConfigHub URLs and compare them to the CLI verification output.

GUI gap: No visual diff between "before setup" and "after setup".

GUI ask: Space creation wizard showing before/after comparison.

**PAUSE.** Wait for the human.

## Stage 3: Check Live Readiness, Do Not Assume It (read-only)

```bash
cd incubator/global-app-layer
./preflight-live.sh <space/target>
./preflight-live.sh <space/target> --json | jq
```

Interpret the result like this:

- target visibility is not the same as readiness
- `set-target.sh` is not the same as apply readiness
- only treat the live path as ready if `applyReady: true`

GUI now: ConfigHub GUI: inspect the chosen target and relevant space before binding anything.

GUI gap: No preflight status shown on target card.

GUI ask: Preflight check result (applyReady: true/false) shown on target card before binding.

**PAUSE.** Wait for the human.

## Stage 4: Bind And Verify The Live Path (mutates ConfigHub, may lead to live apply)

Only do this if the human explicitly wants the live path and preflight passed.

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh <prefix> <space/target>
./verify.sh
```

If the example was already materialized without a target:

```bash
./set-target.sh <space/target>
```

What this proves:

- target binding is explicit and inspectable
- deployment units can be prepared for live delivery

What this does not prove by itself:

- target binding is not the same as successful live apply

GUI now: ConfigHub GUI: inspect the bound units and target relationships.

GUI gap: No unified view showing deployment variant with target binding status.

GUI ask: Deployment variant matrix with target binding status and apply readiness.

**PAUSE.** Wait for the human.

## Capability Check

Before you choose a path, check what is actually available:

```bash
cub version
cub context list --json | jq
cub target list --space "*" --json | jq
```

Important:

- if `cub` is missing or auth is unavailable, stay in preview mode
- if auth works but target listing is empty or irrelevant, use ConfigHub-only mode
- if a real target is visible, run `./preflight-live.sh <space/target>` before you offer the live path

## ArgoCDRenderer Payload Compatibility

- `ArgoCDRenderer` targets expect units containing ArgoCD `Application` CRDs (`apiVersion: argoproj.io/v1alpha1`)
- the raw-manifest examples (`realistic-app`, `single-component`, and friends) are incompatible with `ArgoCDRenderer`
- brownfield-imported `Application` units are the correct Argo renderer proof path
- see [contracts.md](./contracts.md) for the full compatibility matrix

## Smaller Entry Variants

If the user wants the smallest example instead:

```bash
cd incubator/global-app-layer/single-component
./setup.sh --explain
./setup.sh
./verify.sh
```

If the user specifically wants the NVIDIA-shaped example:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh --explain
./setup.sh
./verify.sh
```

If the user wants the full lifecycle instead of only setup and verify, use:

- [whole-journey.md](./whole-journey.md)
