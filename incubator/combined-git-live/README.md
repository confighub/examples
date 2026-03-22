# Combined Git Live

This incubator example adapts the `combined-git-live` flow from `cub-scout` into the official `examples` repo.

It shows the next step after fixture-first compare:

- one Git repo that defines intended app structure
- one live cluster that contains observed workloads
- one read-only `cub-scout combined --suggest --json` result
- a clear split between aligned, Git-only, and cluster-only state

## What This Example Is For

Use this example when you want to show how Git intent and live cluster state can be compared in one place.

This is not a ConfigHub import example. It does not write ConfigHub state by itself.

## Source

This example is adapted from:

- [cub-scout combined-git-live](https://github.com/confighub/cub-scout/tree/main/examples/combined-git-live)

## What It Reads

It reads:

- the copied Git repo under `git-repo/`
- the copied expected output under `expected-output/`
- the current Kubernetes context
- the `cub-scout` binary

## What It Writes

It writes live infrastructure only during setup:

- `payment-dev` namespace
- `payment-prod` namespace
- fixture Deployments in those namespaces

It writes local verification output under `sample-output/` during `./verify.sh`.

It does not write ConfigHub state.

## Read-Only First

```bash
cd incubator/combined-git-live
./setup.sh --explain
./setup.sh --explain-json | jq
kubectl config current-context
```

## Quick Start

```bash
./setup.sh
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating namespaces and applying the fixture Deployments.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It writes local output under `sample-output/`.

`./cleanup.sh` mutates live infrastructure by deleting the fixture namespaces.

## What Success Looks Like

At the cluster level you should see:

- `payment-dev` and `payment-prod` namespaces
- four Flux-labeled Deployments that match Git overlays
- one native `cache-warmer` Deployment that exists only in the cluster

At the compare level you should see:

- four `aligned` entries
- one `git-only` entry for `notifications-service`
- one `cluster-only` entry for `cache-warmer`

## Evidence To Check

Direct cluster evidence:

```bash
kubectl get deployment -n payment-dev
kubectl get deployment -n payment-prod
kubectl get deployment -n payment-prod cache-warmer -o yaml
```

Combined compare evidence:

```bash
./verify.sh
jq '.alignment' sample-output/alignment.json
jq '.alignment[] | select(.status != "aligned")' sample-output/alignment.json
```

## Why This Example Matters

This is a good follow-on from `connect-and-compare`.

It keeps the same compare story, but upgrades one side from fixture-only observed state to a real live cluster.

That makes it useful for:

- Git versus live alignment
- ownership and drift discussions
- explaining why `aligned`, `git-only`, and `cluster-only` are useful categories

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
