# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is offline
  - it does not mutate ConfigHub
  - it does not mutate live infrastructure
  - it scans three local fixtures
- expected anchors:
  - `.example == "demo-data-adt"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == false`
  - `.fixtures | length == 3`

## Local Output Contracts

### `jq '.static.findings' sample-output/dev-eshop.scan.json`

- mutates: no
- proves:
  - the dev fixture has warning findings

### `jq '.static.findings' sample-output/prod-eshop.scan.json`

- mutates: no
- proves:
  - the prod `eshop` fixture is clean in the current scan output

### `jq '.static.findings' sample-output/prod-website.scan.json`

- mutates: no
- proves:
  - the prod `website` fixture has warning findings
