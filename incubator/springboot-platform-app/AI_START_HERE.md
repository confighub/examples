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

## Proof Types

This example has three proof levels:

1. **Structural**: fixture files and contracts (`./setup.sh --explain`)
2. **Local app**: Spring Boot HTTP tests (`cd upstream/app && mvn test`)
3. **ConfigHub-only**: real spaces and units (`./confighub-setup.sh`)

It does not yet:

- bind targets
- apply to a cluster
- prove `lift upstream` or `block/escalate` in ConfigHub

## What You Need Installed

For structural proof:
- `bash`
- `jq`

For local app proof, also:
- Java 21+
- Maven

For ConfigHub-only proof, also:
- `cub` CLI
- `cub auth login` (authenticated context)

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

### D. ConfigHub-only proof

Use:

```bash
./confighub-setup.sh --explain
./confighub-setup.sh
./confighub-verify.sh
```

This creates real ConfigHub spaces and units for dev, stage, and prod.
It does not require a cluster, target, or worker.

### E. Apply-here mutation proof

After running `./confighub-setup.sh`:

```bash
# FEATURE_INVENTORY_RESERVATIONMODE maps to feature.inventory.reservationMode
# via Spring Boot relaxed binding — the running app reads this value
cub function do --space inventory-api-prod --unit inventory-api \
  --change-desc "apply-here: override reservationMode from strict to optimistic for prod rollout" \
  set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
```

To verify the app sees the change:

```bash
cd upstream/app
FEATURE_INVENTORY_RESERVATIONMODE=optimistic mvn spring-boot:run -q -Dspring-boot.run.profiles=prod
# curl -s http://localhost:8081/api/inventory/summary | jq .reservationMode
# expected: "optimistic"
```

### F. Noop target proof

For the full mutation-to-apply workflow without a cluster:

```bash
./confighub-setup.sh --with-targets
./confighub-verify.sh --targets
```

This adds a server worker, Noop targets, binds units, and applies them.
The `apply here` mutation survives re-apply.

### G. Live follow-on

This example does not yet include a real Kubernetes cluster path.

If the human wants a live next step:

- use [`V2-LIVE-PLAN.md`](./V2-LIVE-PLAN.md) for the concrete same-service
  follow-on
- use the GitOps import examples for cluster-first flows
- use `global-app-layer` for ConfigHub-first layered flows
- use `cub-gen` plus `cub-scout` when the question is provenance plus runtime

## Exact Commands To Run

```bash
cd incubator/springboot-platform-app

# Structural proof
./setup.sh --explain
./setup.sh --explain-json | jq '.behaviors'
./verify.sh

# Local app proof
cd upstream/app
mvn test

# ConfigHub-only proof
cd ..
./confighub-setup.sh --explain
./confighub-setup.sh
./confighub-verify.sh
```

## What Mutates What

| Command | Writes |
|---|---|
| `./setup.sh --explain` | nothing |
| `./setup.sh --explain-json` | nothing |
| `./verify.sh` | nothing |
| `cd upstream/app && mvn test` | no ConfigHub or cluster writes; local build output only |
| `./confighub-setup.sh --explain` | nothing |
| `./confighub-setup.sh` | creates 3 spaces and 3 units in ConfigHub |
| `./confighub-setup.sh --with-targets` | + infra space, server worker, Noop targets, apply |
| `./confighub-verify.sh` | nothing (read-only inspection) |
| `./confighub-verify.sh --targets` | nothing (also checks targets and apply status) |
| `./confighub-cleanup.sh` | deletes all spaces with ExampleName label |

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

The structural and local app proofs require no cleanup.

If you ran `./confighub-setup.sh`, clean up with `./confighub-cleanup.sh`.
