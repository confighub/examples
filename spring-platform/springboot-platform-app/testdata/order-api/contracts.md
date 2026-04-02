# Contracts

Stable command outputs for automation and testing.

## Read-Only Contracts

### `./setup.sh --explain-json`

- Mutates: no
- Output: JSON object from [`example-summary.json`](./example-summary.json)
- Stable fields:
  - `example_name`
  - `proof_type`
  - `mutates_confighub`
  - `mutates_live_infra`
  - `reads`
  - `behaviors[].name`
  - `behaviors[].default_route`
- Proves: the example plan before any mutation

### `./verify.sh`

- Mutates: no
- Output: plain text
- Stable success text: `ok: springboot-platform-app fixtures are consistent`

### `./generator/render.sh --explain`

- Mutates: no
- Output: plain text description of the generator transformation

### `./generator/render.sh --explain-field <field>`

- Mutates: no
- Output: plain text showing field lineage and mutation route

## ConfigHub Contracts

### `./confighub-setup.sh --explain-json`

- Mutates: no
- Output: JSON object describing the setup plan
- Stable fields:
  - `proof_type`
  - `mutates_confighub`
  - `mutates_live_infra`
  - `spaces_created`
  - `units_per_space`

### `./confighub-setup.sh`

- Mutates: yes (ConfigHub only)
- Creates:
  - 3 spaces: `order-api-dev`, `order-api-stage`, `order-api-prod`
  - 1 unit per space: `order-api`
- Labels:
  - `ExampleName=springboot-platform-app`
  - `App=order-api`
  - `Environment=<env>`

### `./confighub-setup.sh --with-targets`

- Mutates: yes (ConfigHub and Kubernetes cluster)
- Additional effects:
  - Prod unit bound to Kubernetes target
  - Prod unit applied to cluster
  - Namespace `order-api` created

### `./confighub-verify.sh`

- Mutates: no
- Stable success text: `ok: springboot-platform-app ConfigHub objects are consistent`

### `./confighub-cleanup.sh`

- Mutates: yes (ConfigHub only)
- Deletes all spaces with `Labels.ExampleName = 'springboot-platform-app'`

## Route Bundle Contracts

### `./lift-upstream.sh --explain-json`

- Mutates: no
- Output: JSON object describing the Redis lift-upstream bundle
- Stable fields:
  - `proof_type`
  - `bundle_root`
  - `target_files`
  - `render_diff`

### `./lift-upstream.sh --render-diff`

- Mutates: no
- Output: unified diff showing upstream and ConfigHub changes

### `./lift-upstream-verify.sh`

- Mutates: no
- Stable success text: `ok: springboot-platform-app lift-upstream bundle is consistent`

### `./block-escalate.sh --explain-json`

- Mutates: no
- Output: JSON object describing the datasource boundary
- Stable fields:
  - `proof_type`
  - `current_status`
  - `route_rule`
  - `attempted_env_key`

### `./block-escalate.sh --render-attempt`

- Mutates: no
- Output: shell snippet showing the dry-run override command

### `./block-escalate-verify.sh`

- Mutates: no
- Stable success text: `ok: springboot-platform-app block-escalate bundle is consistent`

## End-to-End Contracts

### `./verify-e2e.sh`

- Mutates: no
- Requires: Kind cluster with deployed app
- Proves:
  - Cluster is reachable
  - Namespace, deployment, and pod exist
  - Service answers `/api/inventory/summary`
  - Reported `reservationMode` comes from actual deployed app
