# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is Flux-based
  - it expects an existing Flux cluster
  - it will apply one GitRepository and one or two Kustomizations
- expected anchors:
  - `.example == "apptique-flux-monorepo"`
  - `.mutates == false`
  - `.fluxRequired == true`
  - `.applies | length >= 2`

## Live Cluster Contracts

### `kubectl get gitrepository -n flux-system apptique-examples -o yaml`

- mutates: no
- proves:
  - the Git source exists in the cluster
  - the source URL and branch are inspectable

### `kubectl get kustomization -n flux-system apptique-dev -o yaml`

- mutates: no
- proves:
  - the dev environment is modeled as its own Flux Kustomization
  - the path to the dev overlay is inspectable

### `kubectl get deployment -n apptique-dev frontend -o yaml`

- mutates: no
- proves:
  - the frontend workload exists
  - the dev deployment is materialized in the target namespace

### `flux get kustomizations -A`

- mutates: no
- proves:
  - Flux reconciliation status is visible for the environment Kustomizations

### `cub-scout trace deployment/frontend -n apptique-dev`

- mutates: no
- proves:
  - the workload can be traced back through Flux ownership to the source objects
- note:
  - this is optional but strongly recommended when `cub-scout` is installed
