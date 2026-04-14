# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- output shape: stable JSON plan for the example
- proves:
  - which spaces will be created
  - which components participate
  - which layered dimensions are used
  - whether a target was provided
- expected anchors:
  - `.example == "global-app-layer-gpu-eks-h100-training"`
  - `.mutates == false`
  - `.spaces | length == 8`
  - `.components | length == 2`
  - `.recipeManifest.unit == "recipe-eks-h100-ubuntu-training-stack"`
  - `.liveConstraints.fluxPrefixMaxLength == 5`
  - `.liveConstraints.knownGoodPrefixExample == "nfx05"`

### `./verify.sh --json`

- mutates: no
- output shape: JSON object with `ok`, `example`, and either verification detail fields or an `error`
- proves:
  - whether the example's ConfigHub state currently passes verification
  - which spaces and units were checked on success
  - that failures are surfaced as structured JSON instead of shell-only stderr
- expected anchors on success:
  - `.ok == true`
  - `.example == "global-app-layer-gpu-eks-h100-training"`
  - `.spacesChecked | length == 8`
  - `.unitsChecked | length == 17`
- expected anchors on failure:
  - `.ok == false`
  - `.error | length > 0`

## ConfigHub State Contracts

## Live Delivery Contract

### `./demo-flux-oci.sh --cleanup-first --target demo-flux/flux-renderer-worker-fluxoci-kubernetes-yaml-cluster`

- mutates: yes
- output shape: human-readable live-run transcript with ConfigHub, Flux, and cluster proof blocks
- proves:
  - the target passed read-only preflight before apply
  - the helper chose or enforced a Flux-safe prefix
  - the layered recipe was materialized and verified
  - the Flux deployment units were approved and applied
  - ConfigHub GUI URLs were surfaced for review
  - Flux `OCIRepository` objects were fetched and Flux `Kustomization` objects reached `Ready=True`
  - the demo workloads became visible in the local cluster
- expected anchors:
  - `Mode: live delivery`
  - `==> Waiting for Flux Kustomization Ready=True`
  - `Flux Kustomizations are Ready=True.`
  - `==> ConfigHub unit status`
  - `==> Flux controller proof`
  - `==> Cluster workload proof`
  - `Completed live Flux OCI demo for prefix `

Known current local truth:

- the proven local Flux lane currently applies the demo workloads into namespace `default`
- `--cleanup-first` is the repeat-run safe path for the dedicated local `demo-flux` cluster
- this proves layered recipe plus Flux OCI delivery, not functional NVIDIA GPU runtime

### `cub space get <prefix>-recipe-eks-h100-ubuntu-training --json`

- mutates: no
- output shape: JSON object containing `Space` plus summary counters
- proves:
  - the recipe space currently exists
  - its `SpaceID` and labels are inspectable
- jq anchor:
  - `cub space get <prefix>-recipe-eks-h100-ubuntu-training --json | jq '.Space | {slug: .Slug, id: .SpaceID, labels: .Labels}'`

### `cub unit get --space <prefix>-recipe-eks-h100-ubuntu-training --json recipe-eks-h100-ubuntu-training-stack`

- mutates: no
- output shape: JSON object containing `Space`, `Unit`, and `UnitStatus`
- proves: the stack-level recipe receipt exists in ConfigHub
- jq anchor:
  - `cub unit get --space <prefix>-recipe-eks-h100-ubuntu-training --json recipe-eks-h100-ubuntu-training-stack | jq '.Unit | {slug: .Slug, revision: .HeadRevisionNum, labels: .Labels}'`

### `cub unit get --space <prefix>-deploy-cluster-a --json gpu-operator-cluster-a`

- mutates: no
- output shape: JSON object containing `Space`, `Unit`, `UnitStatus`, and often `UpstreamUnit`
- proves:
  - the final deployment variant exists
  - target binding is inspectable if present
- jq anchor:
  - `cub unit get --space <prefix>-deploy-cluster-a --json gpu-operator-cluster-a | jq '.Unit | {slug: .Slug, upstreamUnitID: .UpstreamUnitID, targetID: .TargetID, revision: .HeadRevisionNum}'`
- note: in this example the deploy-space units are the direct deployment variants

### `cub unit get --space <prefix>-deploy-cluster-a-flux --json gpu-operator-cluster-a-flux`

- mutates: no
- output shape: JSON object containing `Space`, `Unit`, `UnitStatus`, and often `UpstreamUnit`
- proves:
  - the Flux deployment variant exists
  - target binding is inspectable if present
- jq anchor:
  - `cub unit get --space <prefix>-deploy-cluster-a-flux --json gpu-operator-cluster-a-flux | jq '.Unit | {slug: .Slug, upstreamUnitID: .UpstreamUnitID, targetID: .TargetID, revision: .HeadRevisionNum}'`
- note: this unit is still raw Kubernetes YAML; the Flux delivery contract comes from the target provider type, not from changing the unit payload shape

### `cub unit list --space <prefix>-deploy-cluster-a --quiet --json`

- mutates: no
- output shape: JSON array of objects containing `Space`, `Unit`, `UnitStatus`, and optional `UpstreamUnit`
- proves:
  - which deployment units exist
  - which recipe units they point to
  - current live/not-live status
- jq anchor:
  - `cub unit list --space <prefix>-deploy-cluster-a --quiet --json | jq '.[] | {slug: .Unit.Slug, upstream: .UpstreamUnit.Slug, status: .UnitStatus.Status}'`

### `cub unit list --space <prefix>-deploy-cluster-a-flux --quiet --json`

- mutates: no
- output shape: JSON array of objects containing `Space`, `Unit`, `UnitStatus`, and optional `UpstreamUnit`
- proves:
  - which Flux deployment variant units exist
  - which recipe units they point to
  - current live or not-live status
- jq anchor:
  - `cub unit list --space <prefix>-deploy-cluster-a-flux --quiet --json | jq '.[] | {slug: .Unit.Slug, upstream: .UpstreamUnit.Slug, status: .UnitStatus.Status}'`

### `cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-gpu-eks-h100-training'"`

- mutates: no
- output shape: text tree
- proves: the layered GPU ancestry exists in a human-readable view

### `./.logs/setup.latest.log`

- mutates: no
- output shape: plain text log file
- proves:
  - the setup command completed
  - the summary was printed
  - the GUI URLs are available again later

### `./.logs/verify.latest.log`

- mutates: no
- output shape: plain text log file
- proves:
  - verification stages ran
  - the final success line is durable

### `./.logs/set-target.latest.log`

- mutates: no
- output shape: plain text log file
- proves:
  - the target binding step ran
  - the refreshed GUI URLs and bundle hint are durable

## Expected Output Signals

When `./verify.sh` succeeds, expect:
- the final line `All global-app-layer gpu-eks-h100-training checks passed.`
- no clone-chain error output
- no missing-space or missing-unit errors
- both deployment variant spaces present
