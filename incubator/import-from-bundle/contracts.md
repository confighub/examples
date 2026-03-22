# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is offline
  - it does not mutate ConfigHub
  - it does not mutate live infrastructure
  - it writes local output only
- expected anchors:
  - `.example == "import-from-bundle"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == false`
  - `.source == "bundle"`

## Local Output Contracts

### `jq '.evidence' sample-output/suggestion.json`

- mutates: no
- proves:
  - the dry-run proposal came from a bundle source

### `jq '.proposal' sample-output/suggestion.json`

- mutates: no
- proves:
  - a proposal was generated from the bundle facts

### `jq '.workloads' sample-output/suggestion.json`

- mutates: no
- proves:
  - the workloads discovered in the bundle are inspectable
