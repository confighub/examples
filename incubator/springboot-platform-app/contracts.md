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
