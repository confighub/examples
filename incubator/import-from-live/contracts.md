# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is live-cluster backed
  - it does not mutate ConfigHub by default
  - it will create a local `kind` cluster when run normally
  - it writes local sample output
- expected anchors:
  - `.example == "import-from-live"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`
  - `.clusterType == "kind"`
  - `.writesLocalFilesOnly == false`

## Live Evidence Contracts

### `kubectl get application -n argocd`

- mutates: no
- proves:
  - three Argo `Application` objects exist in the fixture cluster

### `kubectl get deployment -n myapp-dev`

- mutates: no
- proves:
  - the live cluster contains discovered workload resources

### `kubectl get statefulset -n myapp-prod`

- mutates: no
- proves:
  - the live cluster includes Helm-owned leftovers

## Import Proposal Contracts

### `jq '.appSpace' sample-output/suggestion.json`

- mutates: no
- proves:
  - the proposal groups these workloads into one App space

### `jq '.units | length' sample-output/suggestion.json`

- mutates: no
- proves:
  - the proposal contains the expected nine units

### `diff -u expected-output/suggestion.json sample-output/suggestion.normalized.json`

- mutates: no
- proves:
  - the proposal matches the committed expected output for this example
