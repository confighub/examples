# Contracts

## Read-only contracts

### `./deployment-map.json`

- mutates: no
- output: JSON object
- stable fields:
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
- proves:
  - the app/deployment/target model
  - the three target modes
  - the three mutation outcomes

### `./setup.sh --explain-json`

- mutates: no
- output: JSON object
- stable fields:
  - `example_name`
  - `proof_type`
  - `selected_mode`
  - `selected_setup_flag`
  - `default_mode`
  - `mutates_confighub`
  - `mutates_live_infra`
  - `cluster_required`
  - `creates_infra_space`
  - `creates_targets`
  - `applies_units`
  - `deployment_map`
  - `delegates_to`
- proves:
  - which setup mode is in scope
  - whether the selected mode touches live infrastructure
  - where the wrapper delegates implementation work

### `./setup.sh --explain`

- mutates: no
- output: plain text ADT diagram
- stable text anchors:
  - `APP - DEPLOYMENT - TARGET VIEW`
  - `MUTATION OUTCOMES`
  - `Delegation:`
- proves:
  - the human-readable story for the selected mode

### `./demo.sh`

- mutates: no
- output: plain text sections
- stable section headers:
  - `APPLY HERE`
  - `LIFT UPSTREAM`
  - `BLOCK / ESCALATE`
- proves:
  - the three mutation outcomes exist in one app story
  - each outcome points to the matching lower-level flow docs

## Mutating contracts

### `./setup.sh`

- mutates: yes
- default mode:
  - writes ConfigHub spaces, units, noop targets, and applies units
- real-target mode:
  - also touches live Kubernetes infrastructure
- proves:
  - the app-centric wrapper can drive the full underlying example

### `./cleanup.sh`

- mutates: yes
- delegates to:
  - `../springboot-platform-app/confighub-cleanup.sh`
- proves:
  - wrapper cleanup stays aligned with the parent example

## Verification contract

### `./verify.sh`

- mutates: no
- output: plain text success lines
- stable success text:
  - `ok: springboot-platform-app-centric wrapper files are consistent`
- proves:
  - the wrapper file bundle exists
  - `deployment-map.json` remains structurally valid
  - `./setup.sh --explain-json` remains parseable
  - the parent example verification still passes
