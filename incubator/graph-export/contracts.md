# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is live-cluster backed
  - it does not mutate ConfigHub
  - it will create a local `kind` cluster when run normally
  - it writes graph export artifacts locally
- expected anchors:
  - `.example == "graph-export"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`
  - `.clusterType == "kind"`
  - `.writesLocalFilesOnly == false`

## Graph Export Contracts

### `jq '.schema_version' sample-output/graph.json`

- mutates: no
- proves:
  - the canonical export uses the stable `graph.v1` schema

### `jq '[.nodes[].kind] | unique' sample-output/graph.json`

- mutates: no
- proves:
  - the export contains the expected workload kinds for this example

### `jq '[.edges[].type] | unique' sample-output/graph.json`

- mutates: no
- proves:
  - the export contains relationship edges

### `head -n 1 sample-output/graph.dot`

- mutates: no
- proves:
  - DOT rendering was generated

### `head -n 1 sample-output/graph.svg`

- mutates: no
- proves:
  - SVG rendering was generated

### `head -n 1 sample-output/graph.html`

- mutates: no
- proves:
  - HTML rendering was generated
