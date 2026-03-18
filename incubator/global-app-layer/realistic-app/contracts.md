# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- output shape: stable JSON plan for the example
- proves:
  - which spaces will be created
  - which components participate
  - whether a target was provided
- expected anchors:
  - `.example == "global-app-layer-realistic-app"`
  - `.mutates == false`
  - `.spaces | length == 5`
  - `.components | length == 3`
  - `.recipeManifest.unit == "recipe-us-staging-realistic-app"`

## ConfigHub State Contracts

### `cub unit get --space <prefix>-recipe-us-staging --json recipe-us-staging-realistic-app`

- mutates: no
- output shape: JSON array containing `[space, unit, unit-status]`
- proves: the app-level recipe receipt exists in ConfigHub
- jq anchor:
  - `cub unit get --space <prefix>-recipe-us-staging --json recipe-us-staging-realistic-app | jq '.[1] | {slug: .Slug, revision: .HeadRevisionNum, labels: .Labels}'`

### `cub unit get --space <prefix>-deploy-cluster-a --json backend-cluster-a`

- mutates: no
- output shape: JSON array containing `[space, unit, unit-status]`
- proves:
  - the final deployment variant exists
  - target binding is inspectable if present
- jq anchor:
  - `cub unit get --space <prefix>-deploy-cluster-a --json backend-cluster-a | jq '.[1] | {slug: .Slug, upstreamUnitID: .UpstreamUnitID, targetID: .TargetID, revision: .HeadRevisionNum}'`

### `cub unit list --space <prefix>-deploy-cluster-a --quiet --json`

- mutates: no
- output shape: JSON array of objects containing `Space`, `Unit`, `UnitStatus`, and optional `UpstreamUnit`
- proves:
  - which deployment units exist
  - which recipe units they point to
  - current live/not-live status
- jq anchor:
  - `cub unit list --space <prefix>-deploy-cluster-a --quiet --json | jq '.[] | {slug: .Unit.Slug, upstream: .UpstreamUnit.Slug, status: .UnitStatus.Status}'`

### `cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-realistic-app'"`

- mutates: no
- output shape: text tree
- proves: the layered ancestry exists in a human-readable view

## Expected Output Signals

When `./verify.sh` succeeds, expect:
- the final line `All global-app-layer realistic-app checks passed.`
- no clone-chain error output
- no missing-space or missing-unit errors
