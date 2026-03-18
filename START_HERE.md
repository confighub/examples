# Start Here

This is the human entry point for the `confighub/examples` repo.

If you want the shortest path to understanding, use this order:

1. look at one example that does not need a cluster
2. look at one example that teaches the core ConfigHub object model
3. only then move to the bigger layered or fleet-style examples

## First Path: No Cluster Required

Start with [`promotion-demo-data`](./promotion-demo-data/README.md).

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

## Second Path: Learn the Core Object Model

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

## Third Path: Pick the Right Worked Example

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
