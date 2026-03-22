# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is file-based
  - it does not mutate ConfigHub
  - it does not mutate live infrastructure
  - it writes local output only
- expected anchors:
  - `.example == "lifecycle-hazards"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == false`
  - `.writesLocalFilesOnly == true`

## Hook Inventory Contracts

### `jq '.count' sample-output/hooks.json`

- mutates: no
- proves:
  - the example contains four hook resources

### `jq '.hooks[] | select(.name == "db-migrate") | .mappedPhase' sample-output/hooks.json`

- mutates: no
- proves:
  - the mixed Helm hook maps to `PostSync`

## Lifecycle Hazard Contracts

### `jq '.lifecycleHazards.summary' sample-output/lifecycle-scan.json`

- mutates: no
- proves:
  - the lifecycle hazard counts match the fixture intent

### `jq '.lifecycleHazards.findings[] | .rule' sample-output/lifecycle-scan.json`

- mutates: no
- proves:
  - the example surfaces both `helm-hook-ambiguity` and `postsync-idempotency-risk`

### `jq '.static.findings[] | .resource_name' sample-output/lifecycle-scan.json`

- mutates: no
- proves:
  - the general static scanner still runs alongside lifecycle-specific analysis
