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
- output shape: JSON object for the recipe manifest unit
- proves: the app-level recipe receipt exists in ConfigHub

### `cub unit get --space <prefix>-deploy-cluster-a --json backend-cluster-a`

- mutates: no
- output shape: JSON object for one deployment unit
- proves:
  - the final deployment variant exists
  - target binding is inspectable if present

### `cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-realistic-app'"`

- mutates: no
- output shape: text tree
- proves: the layered ancestry exists

## Expected Output Signals

When `./verify.sh` succeeds, expect:
- the final line `All global-app-layer realistic-app checks passed.`
- no clone-chain error output
- no missing-space or missing-unit errors
