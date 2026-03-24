# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is fixture-backed and no-cluster
  - it does not mutate ConfigHub
  - it reads a copied debug bundle
  - it writes only local sample-output artifacts
- expected anchors:
  - `.example == "artifact-workflow"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == false`
  - `.source == "fixtures/debug-bundle"`

## Inspect Contract

### `jq '{target, gitContext, contents}' sample-output/bundle-inspect.json`

- mutates: no
- proves:
  - the bundle preserves target metadata and contents summary

## Replay Contract

### `jq '.[] | {severity, field, expected, actual}' sample-output/bundle-replay-drift.json`

- mutates: no
- proves:
  - the drift finding can be replayed offline
  - the replay summary preserves the warning severity and affected object

## Summarize Contract

### `jq '.' sample-output/bundle-summarize.json`

- mutates: no
- proves:
  - the same bundle can be rendered into a structured summary payload for downstream use
  - Git context and warning-level risk signals are available to the summarizer
