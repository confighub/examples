# Contracts

This file documents the safest stable inspection paths for `gitops-import-argo`.

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- proves:
  - which default steps will run
  - which optional steps exist
  - which local and live assets will be written
  - which ArgoCD host port will be used for local access
- expected anchors:
  - `.example == "gitops-import-argo"`
  - `.mutates == false`
  - `.argocdHostPort`
  - `.steps`
  - `.optionalSteps`
  - `.writes`

### `./verify.sh`

- mutates: no
- output shape: plain text
- proves:
  - the cluster is reachable
  - the `argocd` namespace exists
  - the default guestbook Applications exist and can become `Synced` plus `Healthy`
  - guestbook workloads are listable in the `guestbook` namespace
  - the local worker pid and log are visible if present
  - the local ArgoCD port-forward pid is visible if present

### `kubectl get applications -n argocd`

- mutates: no
- output shape: Kubernetes table output
- proves: ArgoCD `Application` resources are present in the cluster

### `cub target list --space <space> --json`

- mutates: no
- output shape: JSON array
- proves: worker targets are visible in the selected ConfigHub space

### `cub-scout gitops status`

- mutates: no
- output shape: terminal status view
- proves:
  - which GitOps objects appear healthy or unhealthy live
  - GitOps controller-side ownership and status interpretation
- note: this is a live ownership and GitOps context view, not a ConfigHub state view

### `cub-scout map list`

- mutates: no
- output shape: terminal table view
- proves:
  - how live resources are classified by ownership
  - whether a workload appears Argo-managed, Helm-managed, Flux-managed, or native
- note: this is especially useful in brownfield or mixed-management clusters

## ConfigHub Contracts

### `cub gitops discover --space <space> worker-kubernetes-yaml-cluster --json`

- mutates: yes, ConfigHub only
- output shape: JSON resource list
- proves:
  - discover ran against the Kubernetes target
  - GitOps resources were found and serialized into ConfigHub discover state

### `cub gitops import --space <space> worker-kubernetes-yaml-cluster worker-argocdrenderer-kubernetes-yaml-cluster --wait`

- mutates: yes, ConfigHub only
- output shape: text
- proves:
  - renderer and wet units were created
  - the renderer stage completed or failed visibly

### `cub unit list --space <space> --json`

- mutates: no
- output shape: JSON array
- proves:
  - imported units exist in the selected space
  - `-dry` and `-wet` units can be inspected directly

### `cub unit-action get --space <space> <unit> <queued-operation-id> --json`

- mutates: no
- output shape: JSON object
- proves:
  - the renderer action result
  - any error details returned by the import or render flow

## Evidence Boundary

This example can prove three different kinds of evidence:

- ConfigHub evidence
- direct cluster evidence
- optional `cub-scout` live ownership and GitOps context evidence

Import and renderer evidence do not, by themselves, prove live workload reconciliation.

If runtime behavior matters, compare all three surfaces instead of relying on only one.
