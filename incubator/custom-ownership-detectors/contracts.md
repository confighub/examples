# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is live-cluster backed
  - it does not mutate ConfigHub
  - it uses a copied detectors file instead of `~/.cub-scout/detectors.yaml`
  - it will create a local `kind` cluster when run normally
- expected anchors:
  - `.example == "custom-ownership-detectors"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`
  - `.clusterType == "kind"`
  - `.detectorsSource == "local-file"`

## Live Evidence Contracts

### `jq '.[] | {name, owner}' sample-output/map.json`

- mutates: no
- proves:
  - the custom owners appear in `cub-scout map list --json`
  - the unlabeled resource remains `Native`

### `jq '.owner' sample-output/payments-api.explain.json`

- mutates: no
- proves:
  - `explain` returns `Internal Platform` for the labeled Deployment

### `jq '.owner' sample-output/infra-ui.explain.json`

- mutates: no
- proves:
  - `explain` returns `Pulumi` for the annotated Deployment

### `jq '.error' sample-output/payments-api.trace.json`

- mutates: no
- proves:
  - `trace` detects the custom owner and reports that full chain resolution is not available for custom owners
