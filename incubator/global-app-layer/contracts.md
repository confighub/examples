# Contracts

This file documents the safest stable inspection paths for `global-app-layer`.

## Read-Only Contracts

### `./find-runs.sh --json`

- mutates: no
- output shape: JSON array of discovered runs grouped by example labels
- proves: which `global-app-layer` runs currently exist in ConfigHub without guessing prefixes

### `./setup.sh --explain-json`

- mutates: no
- output shape: stable JSON plan for the example setup flow
- proves:
  - which spaces will be created
  - which components participate
  - which layer sequence is used
  - whether a target was provided
- expected anchors:
  - `.example`
  - `.mutates == false`
  - `.spaces`
  - `.components`
  - `.recipeManifest`

### `cub context list --json`

- mutates: no
- output shape: JSON array of available contexts
- proves: whether the current shell has a usable ConfigHub CLI context
- note: for the current context, use `cub context list` plain output and look for the `CURRENT` marker

### `cub target list --space "*" --json`

- mutates: no
- output shape: JSON array of targets visible to the current context
- proves: whether the optional live-delivery path is even visible
- note: this does **not** prove the worker is ready for apply

### `./preflight-live.sh <space/target> --json`

- mutates: no
- output shape: JSON object
- proves:
  - whether the target exists
  - whether the target maps to direct apply or Argo-render-style delivery
  - whether the attached worker is actually ready for apply
- expected anchors:
  - `.mutates == false`
  - `.targetExists == true`
  - `.providerType`
  - `.deliveryMode`
  - `.bridgeWorker.slug`
  - `.applyReady`
  - `.reasons`
- note: this proves worker/target readiness, not payload compatibility; a raw Kubernetes unit can still be incompatible with an `ArgoCDRenderer` target that expects an Argo CD `Application`
- note: for `ArgoCDRenderer`, readiness means the renderer target is reachable, not that Argo-managed workload sync is proven

### `./.logs/setup.latest.log`

- mutates: no
- output shape: plain text log file written by `./setup.sh`
- proves:
  - the exact CLI sequence that just ran
  - the printed GUI URLs for the created spaces and units
  - the summary and next steps are durable, not only in scrollback

### `./.logs/verify.latest.log`

- mutates: no
- output shape: plain text log file written by `./verify.sh`
- proves:
  - which verification stages ran
  - whether verification reached the final success line

### `./.logs/set-target.latest.log`

- mutates: no
- output shape: plain text log file written by `./set-target.sh`
- proves:
  - which target ref was bound
  - the refreshed bundle hint and GUI URLs were printed again

## ConfigHub State Contracts

### `cub space get <space> --json`

- mutates: no
- output shape: JSON object containing `Space` plus summary counters
- proves:
  - the space currently exists
  - its `SpaceID` and labels are inspectable
- jq anchor:
  - `cub space get <space> --json | jq '.Space | {slug: .Slug, id: .SpaceID, labels: .Labels}'`

### `cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-realistic-app'"`

- mutates: no
- output shape: text tree
- proves: the layered ancestry for the selected example exists in ConfigHub

### `cub unit get --space <recipe-space> --json <recipe-manifest-unit>`

- mutates: no
- output shape: JSON object containing `Space`, `Unit`, and `UnitStatus`
- proves: the package created the explicit recipe receipt for the assembled layered app
- jq anchor:
  - `cub unit get --space <recipe-space> --json <recipe-manifest-unit> | jq '.Unit | {slug: .Slug, revision: .HeadRevisionNum, labels: .Labels}'`

### `cub unit get --space <deploy-space> --json <deploy-unit>`

- mutates: no
- output shape: JSON object containing `Space`, `Unit`, `UnitStatus`, and often `UpstreamUnit`
- proves:
  - the final deployment-specific unit exists
  - target binding can be inspected if present
  - the current intended state is materialized in ConfigHub
- jq anchor:
  - `cub unit get --space <deploy-space> --json <deploy-unit> | jq '.Unit | {slug: .Slug, upstreamUnitID: .UpstreamUnitID, targetID: .TargetID, revision: .HeadRevisionNum}'`

### `cub unit list --space <deploy-space> --quiet --json`

- mutates: no
- output shape: JSON array of objects containing `Space`, `Unit`, `UnitStatus`, and optional `UpstreamUnit`
- proves:
  - which deployment units exist
  - which upstream units they point to
  - current live/not-live status
- jq anchor:
  - `cub unit list --space <deploy-space> --quiet --json | jq '.[] | {slug: .Unit.Slug, upstream: .UpstreamUnit.Slug, status: .UnitStatus.Status}'`

## Target/Payload Compatibility

Not all units work with all targets. The payload format must match the target's expectations:

| Target Type | Expected Payload | Error If Mismatched |
|-------------|------------------|---------------------|
| **Kubernetes** | Any raw K8s manifest (Deployment, Service, Namespace, etc.) | N/A - accepts all valid YAML |
| **ArgoCDRenderer** | ArgoCD `Application` CRD only (`apiVersion: argoproj.io/v1alpha1`) | `failed to parse Application: expected apiVersion argoproj.io/v1alpha1, got <actual>` |

### Which Examples Work With Which Targets

| Example | Payload Type | Kubernetes Target | ArgoCDRenderer Target |
|---------|--------------|-------------------|----------------------|
| `realistic-app` | Raw manifests | ✅ Compatible | ❌ Incompatible |
| `single-component` | Raw manifests | ✅ Compatible | ❌ Incompatible |
| `frontend-postgres` | Raw manifests | ✅ Compatible | ❌ Incompatible |
| `gpu-eks-h100-training` | Raw manifests | ✅ Compatible | ❌ Incompatible |
| Brownfield-imported Application units | Application CRD | ✅ Compatible | ✅ Compatible |

### Proving ArgoCDRenderer Works

To prove ArgoCDRenderer delivery, use **brownfield-imported Application units**, not the raw-manifest examples:

```bash
# These units contain Application CRDs and work with ArgoCDRenderer:
cub unit get --space gitops-import-test --data-only argocd-cubbychat-Application-dry | head -5
# apiVersion: argoproj.io/v1alpha1
# kind: Application

# Apply through ArgoCDRenderer:
cub unit set-target --space gitops-import-test argocd-cubbychat-Application-dry gitops-import-test/worker-argocdrenderer-kubernetes-yaml-cluster
cub unit approve --space gitops-import-test argocd-cubbychat-Application-dry
cub unit apply --space gitops-import-test argocd-cubbychat-Application-dry
```

### Why e2e/deliver-argo.sh Is Not ArgoCDRenderer Proof

The `e2e/deliver-argo.sh` helper is a **hybrid** path:
1. Exports raw manifests to `.gitops-stage/`
2. Applies them via `kubectl apply` (direct, not Argo)
3. Creates an ArgoCD Application for drift detection only

This does not prove ArgoCDRenderer delivery. It proves kubectl delivery with Argo visibility.

## Expected Output Signals

When a run succeeds in ConfigHub-only mode, expect:
- a shared prefix across all created spaces
- one recipe manifest in the recipe space
- `verify.sh` exiting successfully
- `verify.sh` printing a final `All ... checks passed.` line

When the live path also succeeds, expect:
- `./preflight-live.sh <space/target>` to report `applyReady: true`
- `./preflight-live.sh <space/target> --json` to make the delivery mode explicit (`direct` vs `argocd-render`)
- target binding visible on deployment units
- successful `cub unit apply`
- for direct targets: resulting live state visible via ConfigHub and the cluster target
- for delegated targets: resulting live state visible via ConfigHub, the delegated agent, and the cluster target
