# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `component`, `mutates_confighub`,
  `mutates_live_infra`, `outputs`
- proves:
  - the example is offline
  - it models the `payments-platform` component
  - it writes local RBAC outputs only when run normally

### `./setup.sh --explain`

- mutates: no
- output shape: plain text
- stable text anchors: `payments-platform`, `prod-us`, `sample-output`
- proves:
  - the example can be inspected before any local output is written
  - the human can see which component and variants are modeled

## Local Output Contracts

### `jq '.component' sample-output/component-map.json`

- mutates: no
- proves:
  - the generated component map is for `payments-platform`

### `jq '.unitsByPiece' sample-output/snapshot.json`

- mutates: no
- proves:
  - Redis, `payments-api`, and RBAC guardrails are separate pieces in the same
    product shape

### `jq '.grants' sample-output/who-can-get-secrets-prod-us.json`

- mutates: no
- proves:
  - the RBAC question can be asked across Redis and the custom API in one
    `payments-platform/prod-us` view

### `jq '.findings' sample-output/findings.json`

- mutates: no
- proves:
  - the example surfaces an RBAC finding instead of hiding it inside YAML

### `jq '.proposedEdits' sample-output/proposed-edit.json`

- mutates: no
- proves:
  - the hardening change is represented as a dry-run edit requiring human review

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `Redis payments-platform RBAC guardrail checks passed`
- proves:
  - all generated local outputs match the intended payments/Redis/RBAC shape
