# Start Here

This is the human entry point for the `confighub/examples` repo.

If you want the shortest path to understanding, use this order:

1. look at one no-cluster evidence or offline import example
2. look at one live brownfield discovery example
3. look at one app-style GitOps layout example
4. look at one live GitOps import example with direct evidence
5. look at one worker extension example
6. look at one example that teaches the core ConfigHub object model
7. only then move to the bigger layered or fleet-style examples

## First Path: No Cluster Required

Start here if you want visible value quickly without depending on a live cluster:

- [`connect-and-compare`](./incubator/connect-and-compare/README.md)
- [`import-from-bundle`](./incubator/import-from-bundle/README.md)
- [`connected-summary-storage`](./incubator/connected-summary-storage/README.md)
- [`artifact-workflow`](./incubator/artifact-workflow/README.md)
- [`graph-export`](./incubator/graph-export/README.md)
- [`incubator/fleet-import`](./incubator/fleet-import/README.md)
- [`incubator/demo-data-adt`](./incubator/demo-data-adt/README.md)

Why:

- they show evidence, compare, reporting, import proposal, offline bundle replay, aggregation, and scanning
- they also include one small shareable topology artifact path
- they are easy to review
- they make mutation boundaries obvious
- they answer “why does this matter?” quickly

Typical flow:

```bash
cd incubator/connect-and-compare
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

For the reporting sibling:

```bash
cd incubator/connected-summary-storage
./setup.sh --explain
./setup.sh
./verify.sh
```

For the offline bundle sibling:

```bash
cd incubator/artifact-workflow
./setup.sh --explain
./setup.sh
./verify.sh
```

For the topology-artifact sibling:

```bash
cd incubator/graph-export
./setup.sh --explain
./setup.sh
./verify.sh
```

## Second Path: Brownfield Discovery From A Live Cluster

If you already have a cluster and want a dry-run ConfigHub proposal before any ConfigHub mutation, use:

- [`import-from-live`](./incubator/import-from-live/README.md)

Why:

- it starts from live cluster reality
- it keeps the first ConfigHub mutation optional
- it is the cleanest single-player bridge from "what is running?" to "what would ConfigHub organize?"

Typical flow:

```bash
cd incubator/import-from-live
./setup.sh --explain
./setup.sh
./verify.sh
```

## App-Centric Mutation Story

If you want to understand how ConfigHub routes operational changes through one app, use:

- [`incubator/springboot-platform-app-centric`](./incubator/springboot-platform-app-centric/README.md)

Why:

- it shows one app (`inventory-api`) with three deployments (dev, stage, prod)
- it shows three target modes: unbound, noop, real
- it shows the three mutation outcomes: apply here, lift upstream, block/escalate
- it works out of the box with noop targets (no cluster required)

Typical flow:

```bash
cd incubator/springboot-platform-app-centric
./setup.sh --explain
./setup.sh
./verify.sh
```

## Third Path: Standard GitOps Import Stories

Start with the published GitOps docs and then use the runnable examples in this repo:

- [Official GitOps Import docs](https://docs.confighub.com/get-started/examples/gitops-import/)
- [`incubator/gitops-import-argo`](./incubator/gitops-import-argo/README.md)
- [`incubator/gitops-import-flux`](./incubator/gitops-import-flux/README.md)

Why:

- they show the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge
- they focus on import, rendered manifests, and evidence
- they do not depend on ConfigHub being the workload applier
- they use direct cluster inspection and `cub-scout` as verification layers
- they are now the standard GitOps stories to optimize for first

Use them like this:

- Argo standard: start with the healthy guestbook path in [`incubator/gitops-import-argo`](./incubator/gitops-import-argo/README.md). Add the brownfield contrast fixtures only after the guestbook path has created value.
- Flux standard: start with the healthy `podinfo` path in [`incubator/gitops-import-flux`](./incubator/gitops-import-flux/README.md). Add the D2 contrast fixtures only after `podinfo` has created value.
- Readiness bar: if either story cannot create value in 5-10 minutes, it is not ready as the standard front door.

Typical flow:

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh
./verify.sh
```

For the Flux sibling:

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh
./verify.sh
```

## Fourth Path: App-Style GitOps Layout

If you want an app-style GitOps layout rather than an import flow, use:

- [`apptique-flux-monorepo`](./incubator/apptique-flux-monorepo/README.md)
- [`apptique-argo-applicationset`](./incubator/apptique-argo-applicationset/README.md)

Why:

- it is the cleanest incubator "one app, multiple environments" GitOps example in the repo
- it shows one base plus two environment overlays
- it is self-contained and live-validated
- it now has both a Flux and Argo incubator path

Typical flow:

```bash
cd incubator/apptique-flux-monorepo
./setup.sh --explain
./setup.sh --with-prod
./verify.sh --with-prod
```

For the Argo sibling:

```bash
cd incubator/apptique-argo-applicationset
./setup.sh --explain
./setup.sh
./verify.sh
```

## Fifth Path: Worker Extensibility

If you want to understand how ConfigHub workers are built and extended, go to:

- [`custom-workers/hello-world-bridge`](./custom-workers/hello-world-bridge/README.md)
- [`custom-workers/hello-world-function`](./custom-workers/hello-world-function/README.md)
- [`custom-workers/kube-score`](./custom-workers/kube-score/README.md)
- [`custom-workers/kyverno`](./custom-workers/kyverno/README.md)
- [`custom-workers/kyverno-server`](./custom-workers/kyverno-server/README.md)
- [`custom-workers/opa-gatekeeper`](./custom-workers/opa-gatekeeper/README.md)

These show simple bridge and function workers, plus policy and validation examples using the SDK as normal Go modules.

## Sixth Path: Stable ConfigHub Model

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

## Seventh Path: Learn The Core Object Model

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

## Eighth Path: Pick The Right Layered Worked Example

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
