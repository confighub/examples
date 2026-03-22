# AI Start Here

Use this page when you want to drive `springboot-platform-app` safely with an AI
assistant.

## What This Example Is For

This is a structural Spring Boot platform/app example for the authority vs
provenance model, with a locally runnable upstream app.

It demonstrates one app, `inventory-api`, with three routed outcomes:

- `apply here`
- `lift upstream`
- `block/escalate`

## Proof Type

This example is structural proof only.

It does not:

- create ConfigHub spaces
- create units
- bind targets
- apply to a cluster

It does include a real upstream Spring Boot app and HTTP-level tests.

## What You Need Installed

- `bash`
- `jq`

If you want to run the local app or HTTP tests, also install:

- Java 21+
- Maven

You do not need `cub` or `cub auth login` for this example.

## Safe First Steps

Start read-only:

```bash
cd incubator/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq
./verify.sh
```

These commands do not mutate ConfigHub or live infrastructure.

## Capability Branching

### A. Preview only

This is the default and recommended path.

Use:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
```

### B. Structural verification

Use:

```bash
./verify.sh
```

This checks that the fixture files and machine-readable contract stay aligned.

### C. Local app HTTP proof

Use:

```bash
cd upstream/app
mvn test
```

These tests start the app on a random local port and call the HTTP API.

### D. Live follow-on

This example does not include a live path.

If the human wants a live next step after understanding the model:

- use [`V2-LIVE-PLAN.md`](./V2-LIVE-PLAN.md) for the concrete same-service
  follow-on
- use the GitOps import examples for cluster-first flows
- use `global-app-layer` for ConfigHub-first layered flows
- use `cub-gen` plus `cub-scout` when the question is provenance plus runtime

## Exact Commands To Run

```bash
cd incubator/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq '.behaviors'
./verify.sh
cd upstream/app
mvn test
```

## What Mutates What

| Command | Writes |
|---|---|
| `./setup.sh --explain` | nothing |
| `./setup.sh --explain-json` | nothing |
| `./verify.sh` | nothing |
| `cd upstream/app && mvn test` | no ConfigHub or cluster writes; local build output only |

## What Success Looks Like

You should be able to say clearly:

- which files are upstream app inputs
- which files are upstream platform policy
- which files represent the materialized operational shape
- that the tests call the app over HTTP, not only Java methods
- which requests are direct ConfigHub mutations
- which requests should be routed upstream
- which requests are blocked or escalated

## Cleanup

No cleanup is required.
