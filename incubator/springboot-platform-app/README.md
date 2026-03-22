# Spring Boot Platform/App Example

This incubator example is a clear, static Spring Boot story for the authority
vs provenance model.

It uses one Heroku-style service, `inventory-api`, with `dev`, `stage`, and
`prod` deployments.

- the app team owns Spring Boot code and upstream app inputs
- the platform team owns runtime policy and managed boundaries
- ConfigHub is where operational config becomes authoritative, mutable, and
  inspectable

## Stack And Scenario

This example is for:

- Spring Boot app teams
- platform teams that render app inputs into operational config
- ConfigHub users who need to explain which changes should mutate in ConfigHub,
  which should be lifted upstream, and which must stay platform-owned

The app is `inventory-api`.

The main question it answers is:

"When I need to change this running app, should that be a direct ConfigHub
mutation, a change routed back to the Spring app inputs, or a platform-owned
field that gets blocked or escalated?"

## What This Proves

This is a structural proof, not a live deployment proof.

It proves:

- a realistic split between app inputs and platform policy
- a clear operational shape that ConfigHub could store authoritatively
- three natural change behaviors in one app lifecycle:
  - `mutable in CH`
  - `lift upstream`
  - `generator-owned`

It does not prove:

- a live cluster apply
- a worker or target integration
- a full end-to-end ConfigHub setup flow

## Prerequisites

You need:

- `bash`
- `jq`

You do not need:

- `cub`
- `cub auth login`
- a worker
- a target
- a cluster

## What This Reads And Writes

What it reads:

- upstream app inputs under [`upstream/app`](./upstream/app)
- upstream platform policy under [`upstream/platform`](./upstream/platform)
- operational shape under [`operational`](./operational)
- example metadata in [`example-summary.json`](./example-summary.json)

What it writes:

- nothing in ConfigHub
- nothing in a cluster
- nothing outside this directory

The provided scripts are read-only.

## Read-Only Preview

Start here:

```bash
cd incubator/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub or live infrastructure.

## File Layout

| Path | Purpose |
|---|---|
| [`upstream/app/pom.xml`](./upstream/app/pom.xml) | App-owned Spring Boot build input |
| [`upstream/app/src/main/resources/application.yaml`](./upstream/app/src/main/resources/application.yaml) | Base Spring config |
| [`upstream/app/src/main/resources/application-prod.yaml`](./upstream/app/src/main/resources/application-prod.yaml) | Prod app overrides |
| [`upstream/platform/runtime-policy.yaml`](./upstream/platform/runtime-policy.yaml) | Platform-owned runtime policy |
| [`upstream/platform/slo-policy.yaml`](./upstream/platform/slo-policy.yaml) | Platform-owned SLO policy |
| [`operational/configmap.yaml`](./operational/configmap.yaml) | Materialized operational config |
| [`operational/deployment.yaml`](./operational/deployment.yaml) | Materialized deployment shape |
| [`operational/service.yaml`](./operational/service.yaml) | Materialized service |
| [`operational/field-routes.yaml`](./operational/field-routes.yaml) | Route rules for field ownership and mutation behavior |
| [`changes/01-mutable-in-ch.md`](./changes/01-mutable-in-ch.md) | Direct ConfigHub mutation example |
| [`changes/02-lift-upstream.md`](./changes/02-lift-upstream.md) | Upstream routing example |
| [`changes/03-generator-owned.md`](./changes/03-generator-owned.md) | Block/escalate example |

## The Three Behaviors

### 1. Mutable in ConfigHub

Request:

"Change `feature.inventory.reservationMode` in prod from `strict` to
`optimistic` for a rollout."

Why it is a direct ConfigHub mutation:

- it is app-owned
- it is per-deployment operational state
- it should survive normal refreshes

See:

- [`changes/01-mutable-in-ch.md`](./changes/01-mutable-in-ch.md)

### 2. Lift upstream

Request:

"This service now needs Redis-backed caching."

Why it should be routed upstream:

- the app contract changes
- Spring app inputs must grow new cache config
- the platform-rendered operational shape changes as a consequence

See:

- [`changes/02-lift-upstream.md`](./changes/02-lift-upstream.md)

### 3. Generator-owned

Request:

"Change `spring.datasource.*` or bypass the managed datasource boundary."

Why it should be blocked or escalated:

- it crosses a platform-owned runtime boundary
- the field is not safe for app-local divergence
- the app team should not mutate it directly

See:

- [`changes/03-generator-owned.md`](./changes/03-generator-owned.md)

## Exact CLI Sequence

```bash
cd incubator/springboot-platform-app

# Human-readable preview
./setup.sh --explain

# Machine-readable contract
./setup.sh --explain-json | jq

# Static consistency checks
./verify.sh
```

All three commands are read-only.

## Expected Output

After `./setup.sh --explain`, you should see:

- the stack and scenario
- proof type: `structural`
- the three behavior categories
- the exact files this example uses

After `./setup.sh --explain-json | jq`, you should see:

- `proof_type: "structural"`
- `mutates_confighub: false`
- `mutates_live_infra: false`
- three behavior entries named:
  - `mutable_in_ch`
  - `lift_upstream`
  - `generator_owned`

After `./verify.sh`, you should see:

- `ok: springboot-platform-app fixtures are consistent`

## Verify It

```bash
./setup.sh --explain-json | jq '.behaviors[].name'
./setup.sh --explain-json | jq '.reads'
./verify.sh
```

## Inspect It In The GUI

There is no GUI or live ConfigHub path for this example yet.

This example is the crisp conceptual starting point, not the live wedge.

If you want the next step after understanding this model:

- use `cub-gen` for source-to-operational provenance
- use `cub-scout` for live runtime inspection
- use the GitOps or layered examples in this repo for live ConfigHub flows

## Troubleshooting

If `jq` is missing:

- install `jq`
- or read [`example-summary.json`](./example-summary.json) directly

If you are not in the right directory:

- run `git rev-parse --show-toplevel`
- then `cd <repo-root>/incubator/springboot-platform-app`

If you expected a live demo:

- this example is intentionally structural and read-only
- use it to explain the model before moving to live GitOps or layered examples

## Cleanup

No cleanup is required.

This example does not create ConfigHub objects or touch live infrastructure.
