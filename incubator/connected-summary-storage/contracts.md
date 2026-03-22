# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is fixture-backed and no-cluster
  - it does not mutate ConfigHub
  - it reads a copied connected summary store
  - it writes only local sample-output artifacts
- expected anchors:
  - `.example == "connected-summary-storage"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == false`
  - `.source == "fixtures/summary-store"`

## Summary List Contracts

### `jq '.count' sample-output/summary-list-all.json`

- mutates: no
- proves:
  - the unfiltered query returned all four fixture records

### `jq '.entries[].type' sample-output/summary-list-kind-dev-prod.json`

- mutates: no
- proves:
  - the filtered query returned the `scan` and `gitops-status` records for `kind-dev/prod`

## Slack Digest Contract

### `jq '{text, blocks}' sample-output/summary-slack-dry-run.json`

- mutates: no
- proves:
  - the stored records can be rendered into a dry-run Slack payload
  - the payload includes human-readable summary text and structured blocks
