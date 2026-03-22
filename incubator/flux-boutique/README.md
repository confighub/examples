# Flux Boutique

This incubator example adapts the `flux-boutique` flow from `cub-scout` into the official `examples` repo.

It shows a small live microservice GitOps path:

- one local `kind` cluster
- Flux controllers installed into the cluster
- one copied fixture with a single `GitRepository` and five `Kustomization` resources
- one `cub-scout map list --json` capture
- one `cub-scout trace deployment/payment --format json` capture

## What This Example Is For

Use this example when you want to show how one Git source can fan out into multiple Flux-managed services and how `cub-scout` makes that ownership visible.

This example does not write ConfigHub state.

## Source

This example is adapted from:

- [cub-scout flux-boutique](https://github.com/confighub/cub-scout/tree/main/examples/flux-boutique)

## What It Reads

It reads:

- the copied `fixtures/boutique.yaml`
- the current Kubernetes context created by `kind`
- the `flux` CLI
- the `cub-scout` binary

## What It Writes

It writes live infrastructure during setup:

- one local `kind` cluster
- Flux controllers in `flux-system`
- boutique resources in namespace `boutique`

It writes local verification output under `sample-output/`:

- `map-list.json`
- `trace-payment.json`
- `flux-kustomizations.txt`

It does not write ConfigHub state.

## Read-Only First

```bash
cd incubator/flux-boutique
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Quick Start

```bash
./setup.sh
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster, installing Flux, applying the boutique fixture, and writing local inspection output.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It checks that the boutique deployments exist, that `map list` sees Flux ownership, and that `trace` can attribute `payment` back to the boutique source.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and clearing local sample output.

## What Success Looks Like

At the cluster level you should see:

- namespace `boutique`
- deployments `frontend`, `cart`, `checkout`, `payment`, and `shipping`
- one GitRepository named `boutique`
- five Flux Kustomizations in namespace `boutique`

At the ownership level you should see:

- `map list --json` entries in namespace `boutique` with `owner == "Flux"`
- a `trace` result for `Deployment/payment` that identifies Flux ownership and the boutique source

The boutique Deployments may still appear as `NotReady` during the first few moments after reconciliation while images are pulling. This example proves the Flux fan-out and ownership chain even if the workloads are not fully available yet.

## Evidence To Check

```bash
./verify.sh
kubectl get deploy -n boutique
cat sample-output/flux-kustomizations.txt
jq '.[] | select(.namespace == "boutique") | {kind, name, owner}' sample-output/map-list.json
jq '{target, summary}' sample-output/trace-payment.json
```

## Why This Example Matters

This gives the incubator set a concrete multi-service GitOps story.

It answers a common question quickly:

- if many services come from one GitRepository, how do we tell which Flux object owns each service and where it came from?

That makes it a good companion to the app-style examples, because it focuses less on repo layout and more on the live ownership fan-out pattern.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
