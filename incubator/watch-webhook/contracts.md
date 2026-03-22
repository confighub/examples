# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is live-cluster backed
  - it does not mutate ConfigHub
  - it will create a local `kind` cluster when run normally
  - it will start a local webhook receiver and write JSONL output locally
- expected anchors:
  - `.example == "watch-webhook"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == true`
  - `.clusterType == "kind"`

## Webhook Capture Contracts

### `jq -r '.event.type' sample-output/webhook-events.jsonl`

- mutates: no
- proves:
  - the receiver captured one or more `cub-scout watch` events

### `jq -e 'select(.event.type == "resource.discovered" and .event.resource.namespace == "watch-demo")' sample-output/webhook-events.jsonl`

- mutates: no
- proves:
  - at least one fixture resource in `watch-demo` produced a `resource.discovered` event

### `jq -e '.receivedAt and .path and .event.resource.kind and .event.resource.name' sample-output/webhook-events.jsonl`

- mutates: no
- proves:
  - the receiver envelope is populated and the event schema has the expected basic fields
