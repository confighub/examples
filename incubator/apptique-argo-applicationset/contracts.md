# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is Argo-based
  - it creates its own `kind` cluster
  - it uses a dedicated kubeconfig under `var/`
  - it installs Argo CD and the ApplicationSet controller
  - it will apply one ApplicationSet
- expected anchors:
  - `.example == "apptique-argo-applicationset"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`
  - `.clusterType == "kind"`
  - `.argoInstalledBySetup == true`
  - `.applies == ["bootstrap/applicationset.yaml"]`

## Live Cluster Contracts

### `kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get applicationset -n argocd apptique -o yaml`

- mutates: no
- proves:
  - the generator exists in the cluster
  - the source repo and path rules are inspectable

### `kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get application -n argocd apptique-dev -o yaml`

- mutates: no
- proves:
  - the dev environment is materialized as its own generated Application

### `kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get deployment -n apptique-dev frontend -o yaml`

- mutates: no
- proves:
  - the frontend workload exists in the generated target namespace

### `cub-scout trace deployment/frontend -n apptique-dev`

- mutates: no
- proves:
  - the workload can be traced back through Argo ownership to the generated Application
- note:
  - this is optional but strongly recommended when `cub-scout` is installed
