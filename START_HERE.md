# Start Here

This is the human entry point for the `confighub/examples` repo.

If you want the shortest path to understanding, use this order:

1. look at one no-cluster evidence or offline import example
2. look at one live brownfield discovery example
3. look at one live GitOps import example with direct evidence
4. look at one worker extension example
5. look at one example that teaches the core ConfigHub object model
6. only then move to the bigger layered or fleet-style examples

## First Path: No Cluster Required

Start here if you want visible value quickly without depending on a live cluster:

- [`connect-and-compare`](./connect-and-compare/README.md)
- [`incubator/import-from-bundle`](./incubator/import-from-bundle/README.md)
- [`incubator/fleet-import`](./incubator/fleet-import/README.md)
- [`incubator/demo-data-adt`](./incubator/demo-data-adt/README.md)

Why:

- they show evidence, compare, import proposal, aggregation, and scanning
- they are easy to review
- they make mutation boundaries obvious
- they answer “why does this matter?” quickly

Typical flow:

```bash
cd connect-and-compare
./setup.sh --explain
./setup.sh
./verify.sh
```

For the offline import sibling:

```bash
cd incubator/import-from-bundle
./setup.sh --explain
./setup.sh
./verify.sh
```

## Second Path: Brownfield Discovery From A Live Cluster

If you already have a cluster and want a dry-run ConfigHub proposal before any ConfigHub mutation, use:

- [`import-from-live`](./import-from-live/README.md)

Why:

- it starts from live cluster reality
- it keeps the first ConfigHub mutation optional
- it is the cleanest single-player bridge from "what is running?" to "what would ConfigHub organize?"

Typical flow:

```bash
cd import-from-live
./setup.sh --explain
./setup.sh
./verify.sh
```

## Third Path: GitOps Import And Evidence

Start with the published GitOps docs and then use the runnable examples in this repo:

- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- [`incubator/gitops-import-argo`](./incubator/gitops-import-argo/README.md)
- [`incubator/gitops-import-flux`](./incubator/gitops-import-flux/README.md)

Why:

- they show the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge
- they focus on import, rendered manifests, and evidence
- they do not depend on ConfigHub being the workload applier
- they use direct cluster inspection and `cub-scout` as verification layers

Typical flow:

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh --with-worker --with-contrast
./verify.sh
```

For the Flux sibling:

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh --with-worker --with-contrast
./verify.sh
```

## Fourth Path: Worker Extensibility

If you want to understand how ConfigHub workers are built and extended, go to:

- [`custom-workers/hello-world-bridge`](./custom-workers/hello-world-bridge/README.md)
- [`custom-workers/hello-world-function`](./custom-workers/hello-world-function/README.md)
- [`custom-workers/kube-score`](./custom-workers/kube-score/README.md)
- [`custom-workers/kyverno`](./custom-workers/kyverno/README.md)
- [`custom-workers/kyverno-server`](./custom-workers/kyverno-server/README.md)
- [`custom-workers/opa-gatekeeper`](./custom-workers/opa-gatekeeper/README.md)

These show simple bridge and function workers, plus policy and validation examples using the SDK as normal Go modules.

## Fifth Path: Stable ConfigHub Model

Then look at [`promotion-demo-data`](./promotion-demo-data/README.md).

Why:

- it is stable
- it is easy to review
- it shows ConfigHub’s multi-environment promotion model
- it does not need a live Kubernetes cluster

Typical flow:

```bash
cd promotion-demo-data
./setup.sh
./cleanup.sh
```

## Sixth Path: Learn The Core Object Model

Then go to the layered examples package:

- [`incubator/global-app-layer`](./incubator/global-app-layer/README.md)

If you are brand new to ConfigHub, start with:

- [`incubator/global-app-layer/00-config-hub-hello-world.md`](./incubator/global-app-layer/00-config-hub-hello-world.md)

Then continue to:

- [`incubator/global-app-layer/confighub-aicr-value-add.md`](./incubator/global-app-layer/confighub-aicr-value-add.md)

That gives you:

- one simple unit in one space
- then one layered recipe model
- then the value-add on top of NVIDIA AICR

When you enter one of the worked examples, use the non-mutating plan first:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh --explain
```

## Seventh Path: Pick The Right Layered Worked Example

Inside [`incubator/global-app-layer`](./incubator/global-app-layer/README.md):

- [`single-component`](./incubator/global-app-layer/single-component/README.md): smallest layered example
- [`frontend-postgres`](./incubator/global-app-layer/frontend-postgres/README.md): small multi-component app
- [`realistic-app`](./incubator/global-app-layer/realistic-app/README.md): fuller app example
- [`gpu-eks-h100-training`](./incubator/global-app-layer/gpu-eks-h100-training/README.md): NVIDIA-style layered recipe

## Prerequisites

At minimum:

```bash
cub auth login
```

For real deployments, you also need:

- a Kubernetes cluster
- a ConfigHub worker
- one or more targets

## If You Prefer an AI-Guided Path

Use:

- [AI_START_HERE.md](./AI_START_HERE.md)

That page starts in read-only mode and gives exact commands plus machine-readable JSON paths before suggesting mutating flows.
