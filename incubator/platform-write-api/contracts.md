# Contracts

## Read-only contracts

### `./setup.sh --explain-json`

- mutates: no
- output: JSON object
- stable fields:
  - `example_name`
  - `proof_type`
  - `mutates_confighub`
  - `mutates_live_infra`
  - `requires_cluster`
  - `spaces_created`
  - `units_per_space`
  - `cleanup`
- proves:
  - what the example creates
  - that the example is ConfigHub-only
  - what cleanup removes

### `./compare.sh --json`

- mutates: no
- output: JSON object keyed by field display name
- stable fields per entry:
  - `display`
  - `route`
  - `values.dev.value`
  - `values.dev.diverged`
  - `values.stage.value`
  - `values.stage.diverged`
  - `values.prod.value`
  - `values.prod.diverged`
- proves:
  - the current cross-environment values for the tracked fields
  - which environment values diverge from upstream defaults
  - which route each tracked field belongs to

### `./field-routes.sh prod --json`

- mutates: no
- output: JSON array
- stable fields per item:
  - `match`
  - `owner`
  - `action`
  - `reason`
  - `currentValue`
  - `environment`
- proves:
  - the route policy for the important field families
  - the current prod example values associated with each route

### `./refresh-preview.sh prod --json`

- mutates: no
- output: JSON array
- stable fields per item:
  - `field`
  - `liveValue`
  - `upstreamValue`
  - `action`
  - `reason`
  - `route`
  - `environment`
- proves:
  - whether a field would be refreshed or preserved
  - why a direct ConfigHub mutation survives or is replaced

### `./lift-upstream.sh --json`

- mutates: no
- output: JSON object
- stable fields:
  - `scenario`
  - `requested_change.field`
  - `routing.route`
  - `routing.reason`
  - `required_changes`
  - `automation_status`
- proves:
  - the change is a lift-upstream case
  - the required source and regenerated-file changes are known
  - automation is still only partial

### `./block-escalate.sh --json`

- mutates: no
- output: JSON object
- stable fields:
  - `scenario`
  - `requested_change.field`
  - `routing.route`
  - `routing.owner`
  - `routing.blocked`
  - `escalation.channels`
  - `audit.logged`
  - `enforcement_status.server_side`
  - `enforcement_status.tracking_issue`
- proves:
  - the field is platform-owned
  - direct mutation is blocked in the current example story
  - server-side enforcement is not yet implemented

## Mutating contracts

### `./setup.sh`

- mutates: yes, ConfigHub only
- creates:
  - `inventory-api-dev`
  - `inventory-api-stage`
  - `inventory-api-prod`
  - one `inventory-api` unit per space
- proves:
  - ConfigHub can store the three-environment inventory service shape

### `./mutate.sh`

- mutates: yes, ConfigHub only
- change:
  - sets `FEATURE_INVENTORY_RESERVATIONMODE=optimistic` on the prod unit
- stable text signals:
  - `Before:`
  - `After:`
  - `Mutation history`
  - `That was the write API.`
- proves:
  - the prod override is stored in ConfigHub
  - the change is auditable and reversible

### `./cleanup.sh`

- mutates: yes, ConfigHub only
- deletes:
  - all spaces with `Labels.ExampleName = 'platform-write-api'`

## Verification contract

### `./verify.sh`

- mutates: no
- output: plain text success line
- stable success text:
  - `ok: platform-write-api example is consistent`
- proves:
  - the AI bundle files exist
  - the JSON contracts parse successfully
  - the delegated fixture scripts are present
