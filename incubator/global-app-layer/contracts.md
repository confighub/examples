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

### `cub target list --space "*" --json`

- mutates: no
- output shape: JSON array of targets visible to the current context
- proves: whether the optional live-delivery path is even possible

## ConfigHub State Contracts

### `cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-realistic-app'"`

- mutates: no
- output shape: text tree
- proves: the layered ancestry for the selected example exists in ConfigHub

### `cub unit get --space <recipe-space> --json <recipe-manifest-unit>`

- mutates: no
- output shape: JSON object for the recipe manifest unit
- proves: the package created the explicit recipe receipt for the assembled layered app

### `cub unit get --space <deploy-space> --json <deploy-unit>`

- mutates: no
- output shape: JSON object for one deployment unit
- proves:
  - the final deployment-specific unit exists
  - target binding can be inspected if present
  - the current intended state is materialized in ConfigHub

## Expected Output Signals

When a run succeeds in ConfigHub-only mode, expect:
- a shared prefix across all created spaces
- one recipe manifest in the recipe space
- `verify.sh` exiting successfully

When the live path also succeeds, expect:
- target binding visible on deployment units
- successful `cub unit apply`
- resulting live state visible via ConfigHub and the cluster target
