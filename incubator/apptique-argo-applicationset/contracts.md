# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is Argo-based
  - it expects an existing Argo CD cluster
  - it will apply one ApplicationSet
- expected anchors:
  - `.example == "apptique-argo-applicationset"`
  - `.mutates == false`
  - `.argoRequired == true`
  - `.applies == ["bootstrap/applicationset.yaml"]`

## Live Cluster Contracts

### `kubectl get applicationset -n argocd apptique -o yaml`

- mutates: no
- proves:
  - the generator exists in the cluster
  - the source repo and path rules are inspectable

### `kubectl get application -n argocd apptique-dev -o yaml`

- mutates: no
- proves:
  - the dev environment is materialized as its own generated Application

### `kubectl get deployment -n apptique-dev frontend -o yaml`

- mutates: no
- proves:
  - the frontend workload exists in the generated target namespace

### `cub-scout trace deployment/frontend -n apptique-dev`

- mutates: no
- proves:
  - the workload can be traced back through Argo ownership to the generated Application
- note:
  - this is optional but strongly recommended when `cub-scout` is installed
