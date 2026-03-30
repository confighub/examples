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
  - `target_modes.noop`
  - `target_modes.real`
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
- Default mode creates:
  - 4 spaces (3 env + 1 infra)
  - 3 units
  - 3 noop targets
  - Applies all units

### `./setup.sh --confighub-only`

- Mutates: yes (ConfigHub only)
- Creates spaces and units without targets

### `./setup.sh --with-targets`

- Mutates: yes (ConfigHub and Kubernetes cluster)
- Deploys to real Kubernetes cluster

### `./cleanup.sh`

- Mutates: yes (ConfigHub only)
- Deletes all spaces with `Labels.ExampleName = 'springboot-platform-app-centric'`

## Verification Contract

### `./verify.sh`

- Mutates: no
- Output: plain text
- Stable success text: `ok: springboot-platform-app-centric is consistent`
