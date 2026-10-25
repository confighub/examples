# Contracts

Stable command outputs for automation and testing.

## Read-Only Contracts

### `./deployment-map.json`

- Mutates: no
- Output: JSON object
- Stable fields:
  - `app.name`
  - `app.source`
  - `deployments[].name`
  - `deployments[].space`
  - `target_modes.unbound`
  - `mutation_outcomes[].name`
  - `mutation_outcomes[].example_field`
  - `mutation_outcomes[].flow`

### `./setup.sh --explain-json`

- Mutates: no
- Output: JSON object
- Stable fields:
  - `example_name` = `springboot-platform-app-centric`
  - `proof_type` = `adt-view`
  - `selected_mode`
  - `mutates_confighub`
  - `mutates_live_infra`
  - `cluster_required`
  - `deployment_map`
  - `evaluation_modes.fast_preview`
  - `evaluation_modes.fast_operational_evaluation`
- Proves: the example plan before any mutation

### `./setup.sh --explain`

- Mutates: no
- Output: plain text ADT diagram
- Stable text anchors:
  - `APP - DEPLOYMENT - TARGET VIEW`
  - `MUTATION OUTCOMES`

### `./demo.sh`

- Mutates: no
- Output: plain text sections
- Stable section headers:
  - `APPLY HERE`
  - `LIFT UPSTREAM`
  - `BLOCK / ESCALATE`

## Mutating Contracts

### `./setup.sh`

- Mutates: yes (ConfigHub)
- Creates:
  - 3 spaces (dev, stage, prod)
  - 3 units
  - No targets
- Proves: the deployment map can be materialized into real ConfigHub objects

### `./cleanup.sh`

- Mutates: yes (ConfigHub only)
- Deletes all spaces with `Labels.ExampleName = 'springboot-platform-app-centric'`

## Verification Contract

### `./verify.sh`

- Mutates: no
- Output: plain text
- Stable success text: `ok: springboot-platform-app-centric fixtures are consistent`
- Proves: fixture integrity and `--explain-json` contract validity
- Does **not** prove: live post-setup ConfigHub state

### `cub space list --where "Labels.ExampleName = 'springboot-platform-app-centric'" --json`

- Mutates: no
- Output: JSON array of spaces
- Stable fields:
  - `.[].Space.Slug`
  - `.[].Space.Labels.ExampleName`
- Proves: the example's created objects can be isolated by label

### `cub function do --space inventory-api-prod --unit inventory-api ... set-env ...`

- Mutates: yes (ConfigHub)
- Output: plain text success message including `Config data changed`
- Proves: the representative `apply-here` path works for this example

### `cub mutation list --space inventory-api-prod --json inventory-api`

- Mutates: no
- Output: JSON array
- Stable fields:
  - `.[].Mutation.MutationNum`
  - `.[].Revision.Source`
  - `.[].Revision.Description`
  - `.[].Revision.CreatedAt`
- Optional or environment-dependent fields:
  - `.[].Mutation.CreatedAt`
  - `.[].Author.Email`
- Proves: mutation history captures mutation order and revision description, and usually exposes a usable timestamp through `Revision.CreatedAt`
- Does **not** prove: every surface will populate actor email or `Mutation.CreatedAt`
