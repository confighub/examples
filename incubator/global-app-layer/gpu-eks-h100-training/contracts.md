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
  - `.spaces | length == 6`
  - `.components | length == 2`
  - `.recipeManifest.unit == "recipe-eks-h100-ubuntu-training-stack"`

## ConfigHub State Contracts

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

### `cub unit list --space <prefix>-deploy-cluster-a --quiet --json`

- mutates: no
- output shape: JSON array of objects containing `Space`, `Unit`, `UnitStatus`, and optional `UpstreamUnit`
- proves:
  - which deployment units exist
  - which recipe units they point to
  - current live/not-live status
- jq anchor:
  - `cub unit list --space <prefix>-deploy-cluster-a --quiet --json | jq '.[] | {slug: .Unit.Slug, upstream: .UpstreamUnit.Slug, status: .UnitStatus.Status}'`

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
