# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is offline
  - it does not mutate ConfigHub
  - it does not mutate live infrastructure
  - it reads two existing cluster import files
- expected anchors:
  - `.example == "fleet-import"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == false`
  - `.inputs | length == 2`

## Local Output Contracts

### `jq '.summary' sample-output/fleet-summary.json`

- mutates: no
- proves:
  - the fleet summary was generated
  - total clusters and total workloads are inspectable

### `jq '.proposal' sample-output/fleet-summary.json`

- mutates: no
- proves:
  - a unified proposal was generated from the two cluster inputs

### `jq '.summary.byApp | to_entries[] | select(.value | length > 1)' sample-output/fleet-summary.json`

- mutates: no
- proves:
  - apps spanning multiple clusters are inspectable
