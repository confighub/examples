# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is Argo-based
  - it expects an existing Argo CD cluster
  - it will apply one root Application
- expected anchors:
  - `.example == "apptique-argo-app-of-apps"`
  - `.mutates == false`
  - `.argoRequired == true`
  - `.applies == ["root/root-app.yaml"]`

## Live Cluster Contracts

### `kubectl get application -n argocd apptique-apps -o yaml`

- mutates: no
- proves:
  - the root Application exists
  - the root source path is inspectable

### `kubectl get application -n argocd apptique-dev -o yaml`

- mutates: no
- proves:
  - the dev environment is materialized as its own child Application

### `kubectl get deployment -n apptique-dev frontend -o yaml`

- mutates: no
- proves:
  - the frontend workload exists in the child target namespace

### `cub-scout trace deployment/frontend -n apptique-dev`

- mutates: no
- proves:
  - the workload can be traced back through the child Application to the root Application
- note:
  - this is optional but strongly recommended when `cub-scout` is installed
