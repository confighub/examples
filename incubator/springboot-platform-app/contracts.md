# Contracts

## Read-only contracts

### `./setup.sh --explain-json`

- mutates: no
- output: JSON object from [`example-summary.json`](./example-summary.json)
- stable fields:
  - `example_name`
  - `proof_type`
  - `mutates_confighub`
  - `mutates_live_infra`
  - `reads`
  - `behaviors[].name`
  - `behaviors[].default_route`
- proves:
  - the example shape
  - the read-only status
  - the three behavior categories

### `./verify.sh`

- mutates: no
- output: plain text success line
- stable success text:
  - `ok: springboot-platform-app fixtures are consistent`
- proves:
  - required files exist
  - the machine-readable contract is internally consistent
  - the local HTTP-test source files are present

## Local app contract

### `cd upstream/app && mvn test`

- mutates: no ConfigHub state, no cluster state
- local effects:
  - Maven target directory
  - local test runtime only
- proves:
  - the Spring Boot app starts in tests
  - the tests call the HTTP API on a random local port
  - the default and `prod` profile responses are observable over HTTP

## ConfigHub-only contracts

### `./confighub-setup.sh --explain-json`

- mutates: no
- output: JSON object describing ConfigHub-only setup plan
- stable fields:
  - `proof_type`
  - `mutates_confighub`
  - `spaces_created`
  - `units_per_space`
  - `labels`
- proves:
  - what ConfigHub objects will be created
  - what the cleanup path is

### `./confighub-setup.sh`

- mutates: yes (ConfigHub only)
- creates:
  - 3 spaces: `inventory-api-dev`, `inventory-api-stage`, `inventory-api-prod`
  - 1 unit per space: `inventory-api`
- labels:
  - `ExampleName=springboot-platform-app`
  - `App=inventory-api`
  - `Environment=<env>`

### `./confighub-verify.sh`

- mutates: no
- output: plain text success line
- stable success text:
  - `ok: springboot-platform-app ConfigHub-only objects are consistent`
- proves:
  - 3 spaces exist with the expected label
  - each space contains an `inventory-api` unit with resources

### `./confighub-setup.sh --with-targets`

- mutates: yes (ConfigHub only, no cluster)
- creates in addition to the base setup:
  - 1 infra space: `inventory-api-infra` with a server worker
  - 1 Noop target per env space
  - binds units to targets
  - applies units to Noop targets
- labels: same as base, plus `AppOwner=Platform` on infra space

### `./confighub-verify.sh --targets`

- mutates: no
- output: plain text success line
- stable success text:
  - `ok: springboot-platform-app ConfigHub objects with targets are consistent`
- proves:
  - 4 spaces exist (infra + 3 envs)
  - each env space has a unit and a target
  - unit status is `Ready` / `Synced`

### `./confighub-cleanup.sh`

- mutates: yes (ConfigHub only)
- deletes all spaces with `Labels.ExampleName = 'springboot-platform-app'`

## Lift-upstream bundle contracts

### `./lift-upstream.sh --explain-json`

- mutates: no
- output: JSON object describing the Redis lift-upstream bundle
- stable fields:
  - `proof_type`
  - `bundle_root`
  - `target_files`
  - `render_diff`
- proves:
  - the durable upstream files that must change
  - the refreshed ConfigHub YAMLs that follow from that change

### `./lift-upstream.sh --render-diff`

- mutates: no
- output: unified diff
- proves:
  - the exact GitHub-ready patch for the Redis caching request
  - the current-to-refreshed delta for upstream app inputs and ConfigHub YAMLs

### `./lift-upstream-verify.sh`

- mutates: no
- output: plain text success line
- stable success text:
  - `ok: springboot-platform-app lift-upstream bundle is consistent`
- proves:
  - the Redis bundle files exist
  - the bundle adds the Redis starter dependency
  - the bundle sets `spring.cache.type=redis`
  - the refreshed ConfigHub YAMLs set `CACHE_BACKEND=redis`

## Block/escalate boundary contracts

### `./block-escalate.sh --explain-json`

- mutates: no
- output: JSON object describing the datasource override boundary
- stable fields:
  - `proof_type`
  - `current_status`
  - `route_rule`
  - `attempted_env_key`
- proves:
  - the example has a concrete boundary artifact for the blocked route
  - the current state is explicitly classified as `not_proven`

### `./block-escalate.sh --render-attempt`

- mutates: no
- output: shell snippet
- proves:
  - the exact dry-run `cub function do` command for the datasource override attempt
  - the example does not invent an escalation mechanism it does not have

### `./block-escalate-verify.sh`

- mutates: no
- output: plain text success line
- stable success text:
  - `ok: springboot-platform-app block-escalate bundle is consistent`
- proves:
  - the route rules mark `spring.datasource.*` as generator-owned
  - the runtime policy declares a managed datasource boundary
  - the rendered attempt stays read-only

## Fixture contracts

### [`operational/field-routes.yaml`](./operational/field-routes.yaml)

- mutates: no
- stable fields:
  - `routes[].match`
  - `routes[].owner`
  - `routes[].defaultAction`
- proves:
  - which fields are direct ConfigHub mutations
  - which fields are routed upstream
  - which fields are platform-owned
