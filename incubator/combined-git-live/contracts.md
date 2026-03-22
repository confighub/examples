# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example will mutate live infrastructure only
  - it does not mutate ConfigHub
  - it applies the local cluster fixtures
- expected anchors:
  - `.example == "combined-git-live"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`

## Live And Compare Contracts

### `kubectl get deployment -n payment-prod cache-warmer -o yaml`

- mutates: no
- proves:
  - a cluster-only workload exists live

### `jq '.alignment' sample-output/alignment.json`

- mutates: no
- proves:
  - the combined result was generated
  - aligned and non-aligned states are inspectable

### `jq '.alignment[] | select(.status != "aligned")' sample-output/alignment.json`

- mutates: no
- proves:
  - the example contains both `git-only` and `cluster-only` evidence
