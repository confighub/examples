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
- proves: whether the optional live-delivery path is even possible

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

### `cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-realistic-app'"`

- mutates: no
- output shape: text tree
- proves: the layered ancestry for the selected example exists in ConfigHub

### `cub unit get --space <recipe-space> --json <recipe-manifest-unit>`

- mutates: no
- output shape: JSON array containing `[space, unit, unit-status]`
- proves: the package created the explicit recipe receipt for the assembled layered app
- jq anchor:
  - `cub unit get --space <recipe-space> --json <recipe-manifest-unit> | jq '.[1] | {slug: .Slug, revision: .HeadRevisionNum, labels: .Labels}'`

### `cub unit get --space <deploy-space> --json <deploy-unit>`

- mutates: no
- output shape: JSON array containing `[space, unit, unit-status]`
- proves:
  - the final deployment-specific unit exists
  - target binding can be inspected if present
  - the current intended state is materialized in ConfigHub
- jq anchor:
  - `cub unit get --space <deploy-space> --json <deploy-unit> | jq '.[1] | {slug: .Slug, upstreamUnitID: .UpstreamUnitID, targetID: .TargetID, revision: .HeadRevisionNum}'`

### `cub unit list --space <deploy-space> --quiet --json`

- mutates: no
- output shape: JSON array of objects containing `Space`, `Unit`, `UnitStatus`, and optional `UpstreamUnit`
- proves:
  - which deployment units exist
  - which upstream units they point to
  - current live/not-live status
- jq anchor:
  - `cub unit list --space <deploy-space> --quiet --json | jq '.[] | {slug: .Unit.Slug, upstream: .UpstreamUnit.Slug, status: .UnitStatus.Status}'`

## Expected Output Signals

When a run succeeds in ConfigHub-only mode, expect:
- a shared prefix across all created spaces
- one recipe manifest in the recipe space
- `verify.sh` exiting successfully
- `verify.sh` printing a final `All ... checks passed.` line

When the live path also succeeds, expect:
- target binding visible on deployment units
- successful `cub unit apply`
- resulting live state visible via ConfigHub and the cluster target
