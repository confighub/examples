# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is live-cluster backed
  - it does not mutate ConfigHub
  - it will create a local `kind` cluster when run normally
  - it writes orphan inventory output locally
- expected anchors:
  - `.example == "orphans"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`
  - `.clusterType == "kind"`

## Orphan Inventory Contracts

### `jq '.[] | select(.name == "debug-nginx") | .owner' sample-output/orphans.json`

- mutates: no
- proves:
  - `debug-nginx` is classified as `Native`

### `jq '.[] | select(.name == "manual-override") | .owner' sample-output/orphans.json`

- mutates: no
- proves:
  - a manually created `ConfigMap` is classified as `Native`

### `jq '.[] | select(.name == "manual-api-key") | .owner' sample-output/orphans.json`

- mutates: no
- proves:
  - a manually created `Secret` is classified as `Native`

### `kubectl get cronjob -n default manual-cleanup`

- mutates: no
- proves:
  - the fixture creates a native CronJob even though the current orphan inventory does not surface it yet

## Native Trace Contract

### `jq '{target, summary}' sample-output/debug-nginx.trace.json`

- mutates: no
- proves:
  - the example captured the current `trace` payload for a representative orphan
  - the JSON names the traced object even if the current owner inference is surprising
