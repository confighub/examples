# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is live-cluster backed
  - it does not mutate ConfigHub
  - it will create a local `kind` cluster and install Flux when run normally
  - it will apply both GitOps-managed and orphan resources
- expected anchors:
  - `.example == "platform-example"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`
  - `.clusterType == "kind"`

## Mixed Ownership Contracts

### `jq '.[] | select(.owner == "Flux")' sample-output/map-list.json`

- mutates: no
- proves:
  - the cluster contains Flux-managed resources

### `jq '.[] | select(.owner == "Native")' sample-output/orphans.json`

- mutates: no
- proves:
  - the cluster contains unmanaged resources that `cub-scout` surfaces as orphans

### `jq '{target, summary}' sample-output/trace-podinfo.json`

- mutates: no
- proves:
  - `Deployment/podinfo` can be traced as a Flux-managed resource with a source and deployer
