# Contracts

This file documents the safest stable inspection paths for `gitops-import-flux`.

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- proves:
  - which default steps will run
  - which optional steps exist
  - which local and live assets will be written
- expected anchors:
  - `.example == "gitops-import-flux"`
  - `.mutates == false`
  - `.steps`
  - `.optionalSteps`
  - `.writes`

### `./verify.sh`

- mutates: no
- output shape: plain text
- proves:
  - the cluster is reachable
  - the `flux-system` namespace exists
  - Flux controllers are listable
  - the real podinfo Flux path exists as the healthy reference path
  - worker targets are visible if present
  - the script can show failing contrast objects without treating them as a harness failure

### `flux get all -A`

- mutates: no
- output shape: Flux table output
- proves: Flux controllers, sources, and deployers are visible to the cluster API

### `kubectl get gitrepositories,kustomizations,helmreleases -A`

- mutates: no
- output shape: Kubernetes table output
- proves: Flux source and deployer objects are present in the cluster

### `cub target list --space <space> --json`

- mutates: no
- output shape: JSON array
- proves: discovery, renderer, and Flux deployment targets are visible in the selected ConfigHub space

### `cub-scout gitops status`

- mutates: no
- output shape: terminal status view
- proves:
  - which Flux objects appear healthy or unhealthy live
  - GitOps controller-side ownership and status interpretation
- note: this is a live ownership and GitOps context view, not a ConfigHub state view

### `cub-scout map list`

- mutates: no
- output shape: terminal table view
- proves:
  - how live resources are classified by ownership
  - whether a workload appears Flux-managed, Helm-managed, or native
- note: this is especially useful in brownfield or mixed-management clusters

## ConfigHub Contracts

### `cub gitops discover --space <space> <kubernetes-target-slug> --json`

- mutates: yes, ConfigHub only
- output shape: JSON resource list
- proves:
  - discover ran against the Kubernetes target
  - Flux deployers were found and serialized into ConfigHub discover state

### `cub gitops import --space <space> <kubernetes-target-slug> <flux-renderer-target-slug> --wait`

- mutates: yes, ConfigHub only
- output shape: text
- proves:
  - renderer and wet units were created
  - the renderer stage completed or failed visibly
  - the healthy `podinfo` path can render successfully even when contrast paths fail for real source reasons

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

## Expected Contrast Outcome

In this example, a mixed result is expected and useful:

- `podinfo` is the healthy reference path
- the `platform-config` Kustomizations are expected to fail when the Git source has no artifact
- the HelmRelease paths are expected to fail when their chart sources are not ready

That is not noise. It is the core value proposition of the example.
