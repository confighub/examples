# AI Start Here

Use this page when you want to drive `springboot-platform-app` safely with an AI
assistant.

## What This Example Is For

This is a structural Spring Boot platform/app example for the authority vs
provenance model.

It demonstrates one app, `inventory-api`, with three change behaviors:

- `mutable in CH`
- `lift upstream`
- `generator-owned`

## Proof Type

This example is structural proof only.

It does not:

- create ConfigHub spaces
- create units
- bind targets
- apply to a cluster

## What You Need Installed

- `bash`
- `jq`

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

### C. Live follow-on

This example does not include a live path.

If the human wants a live next step after understanding the model:

- use the GitOps import examples for cluster-first flows
- use `global-app-layer` for ConfigHub-first layered flows
- use `cub-gen` plus `cub-scout` when the question is provenance plus runtime

## Exact Commands To Run

```bash
cd incubator/springboot-platform-app
./setup.sh --explain
./setup.sh --explain-json | jq '.behaviors'
./verify.sh
```

## What Mutates What

| Command | Writes |
|---|---|
| `./setup.sh --explain` | nothing |
| `./setup.sh --explain-json` | nothing |
| `./verify.sh` | nothing |

## What Success Looks Like

You should be able to say clearly:

- which files are upstream app inputs
- which files are upstream platform policy
- which files represent the materialized operational shape
- which requests are direct ConfigHub mutations
- which requests should be routed upstream
- which requests are blocked or escalated

## Cleanup

No cleanup is required.
