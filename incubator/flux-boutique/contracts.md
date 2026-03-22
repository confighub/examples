# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is live-cluster backed
  - it does not mutate ConfigHub
  - it will create a local `kind` cluster and install Flux when run normally
  - it writes ownership evidence locally
- expected anchors:
  - `.example == "flux-boutique"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`
  - `.clusterType == "kind"`

## Ownership Contracts

### `jq '.[] | select(.namespace == "boutique" and .kind == "Deployment") | .owner' sample-output/map-list.json`

- mutates: no
- proves:
  - the boutique deployments are classified as Flux-managed

### `jq '{target, summary}' sample-output/trace-payment.json`

- mutates: no
- proves:
  - `Deployment/payment` can be traced as a Flux-managed resource
  - the trace JSON names the traced object and includes current ownership summary fields
