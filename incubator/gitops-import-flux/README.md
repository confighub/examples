# GitOps Import Flux

This incubator example is the working Flux import path for the current GitHub + Flux + AI/CLI + ConfigHub wedge.

It sets up a local kind cluster with real Flux, optionally installs a ConfigHub discovery worker and an in-cluster Flux worker with both `fluxrenderer` and `fluxoci`, and demonstrates `cub gitops discover` and `cub gitops import` against Flux `Kustomization` and `HelmRelease` resources.

It also includes an optional contrast layer adapted from `cub-scout` so you can compare two different Flux-shaped situations in one cluster:

- real Flux-reconciled podinfo resources
- D2 brownfield resources with Flux ownership signals and partial controller coverage

## What This Example Is For

Use this example when you want to show that ConfigHub can import GitOps-managed WET configuration from a Flux environment, organize it in ConfigHub, and give you evidence to inspect without requiring ConfigHub to be the workload applier.

This is the import-and-evidence story, not the layered recipe story.

## What It Reads

It reads:

- local setup scripts in `bin/`
- the kind cluster config in `cluster/`
- Flux CLI installation manifests
- real Flux podinfo fixtures in `fixtures/flux/`
- optional D2 brownfield contrast fixtures in `fixtures/contrast/`
- your current `cub` auth and `CUB_SPACE` if you install the workers

## What It Writes

It writes local state:

- `var/<cluster>.kubeconfig`
- `var/discovery-worker.pid`
- `var/discovery-worker.log`

It writes live infrastructure:

- a kind cluster
- Flux controllers in `flux-system`
- real Flux GitRepository and Kustomization resources for podinfo
- optional D2 brownfield fixtures
- optional ConfigHub renderer and Flux OCI worker deployment in the cluster

It writes ConfigHub state only if you choose the worker and import path:

- one discovery worker and one in-cluster Flux worker in the selected space
- targets registered by those workers
- discover, dry, wet, and linked units created by `cub gitops discover` and `cub gitops import`

## Read-Only First

Preview the setup plan before you mutate anything:

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not create a cluster, do not create ConfigHub objects, and do not touch live infrastructure.

## Quick Start

Cluster and Flux only:

```bash
cd incubator/gitops-import-flux
./setup.sh
./verify.sh
```

Cluster, Flux, and optional contrast fixtures:

```bash
./setup.sh --with-contrast
./verify.sh
```

Cluster, Flux, ConfigHub workers, and optional contrast fixtures:

```bash
cub auth login
export CUB_SPACE=<space>
./setup.sh --with-worker --with-contrast
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating a cluster, installing Flux, and applying real Flux fixtures.

`./setup.sh --with-contrast` mutates live infrastructure further by applying D2 brownfield fixtures.

`./setup.sh --with-worker` mutates ConfigHub and live infrastructure by creating ConfigHub workers, starting a local discovery worker process, and installing the in-cluster Flux worker that exposes both `fluxrenderer` and `fluxoci` targets.

`cub gitops discover` mutates ConfigHub only by creating or reusing the discover unit.

`cub gitops import` mutates ConfigHub only by creating renderer units, wet units, and links.

`./cleanup.sh` mutates live infrastructure and local files by deleting the kind cluster, local kubeconfig state, and any local discovery worker process state.

## Access

After setup:

- kubeconfig: `export KUBECONFIG=$PWD/var/gitops-import-flux.kubeconfig`
- Flux overview: `flux get all -A`
- podinfo deployment: `kubectl get deployment -n podinfo podinfo`

## Discover And Import

If you installed the workers and have a valid `CUB_SPACE`, you should see:

- one Kubernetes discovery target from the local discovery worker
- one `fluxrenderer` target from the in-cluster worker
- one `fluxoci` target from the same in-cluster worker

Use those for the import path:

```bash
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" <kubernetes-target-slug> --json | jq
cub gitops import --space "$CUB_SPACE" <kubernetes-target-slug> <flux-renderer-target-slug> --wait
```

The worker installer preloads the Flux controller images and the ConfigHub Flux worker image into the kind cluster before waiting on rollout. That avoids long or flaky first-time image pulls from becoming the main story of the example.

The `fluxoci` target is useful beyond this import example. It is the deployment bridge that raw-manifest examples such as `incubator/global-app-layer/gpu-eks-h100-training` can bind to when they want a Flux-managed deployment variant.

## What Success Looks Like

At the cluster level you should see:

- a reachable kind cluster
- Flux controllers running in `flux-system`
- a ready GitRepository and Kustomization for podinfo
- if `--with-contrast` was used, D2 brownfield Kustomizations and HelmReleases alongside the real podinfo path

At the ConfigHub level, after discover and import, you should see:

- discovered Flux deployers from `cub gitops discover`
- `-dry` and `-wet` units in the selected space
- successful unit actions for the renderer stage
- imported WET configuration available for inspection

In the current contrast setup, the most useful live outcome is mixed on purpose:

- `podinfo` is the healthy reference path
- the `platform-config` Kustomizations fail because their Git source is intentionally missing
- the two HelmRelease paths fail because their Helm chart sources are intentionally not ready

That gives the example an immediate reason to import: one place to see both the good path and the real broken paths.

## Live Verification Model

For LIVE work, use a layered verification model instead of relying on one status surface.

Use direct cluster commands to prove raw runtime facts.

Use ConfigHub commands to prove discover, render, and stored WET configuration facts.

Use `cub-scout` to prove live ownership and GitOps context facts, especially when the cluster contains a mix of:

- resources really created by Flux
- resources that only carry Flux ownership signals
- Helm-managed resources
- native unmanaged resources

That is particularly important for brownfield-style examples. It lets the example show not just that something exists, but who is really managing it.

## Evidence To Check

Direct cluster evidence:

```bash
export KUBECONFIG=$PWD/var/gitops-import-flux.kubeconfig
flux get all -A
kubectl get gitrepositories,kustomizations,helmreleases -A
kubectl get all -A
```

ConfigHub evidence:

```bash
cub target list --space "$CUB_SPACE" --json | jq
cub unit list --space "$CUB_SPACE" --json | jq
cub unit-action list --space "$CUB_SPACE" <unit-slug>
cub unit-action get --space "$CUB_SPACE" <unit-slug> <queued-operation-id> --json | jq
```

Optional `cub-scout` live ownership and GitOps context evidence:

```bash
cub-scout gitops status
cub-scout map list
cub-scout tree ownership
```

Important: import and renderer evidence are not the same thing as proving that a live GitOps controller reconciled workloads. If you need runtime truth, compare ConfigHub evidence and `cub-scout` evidence with direct cluster evidence.

## Known Live Outcome

On the live run used to validate this incubator example:

- `cub gitops discover` found 7 Flux deployers
- `flux-system-podinfo-Kustomization-dry` completed successfully and rendered WET output
- the `platform-config` Kustomizations failed with `GitRepository flux-system/platform-config has no artifact in status`
- the two HelmRelease dry units remained source-blocked until the missing chart sources were resolved

That is a good wedge result. It proves ConfigHub can import and render the healthy path while also surfacing controller-side problems immediately.

## Optional Contrast Path

The optional contrast fixtures are adapted from `cub-scout`. They let you inspect the difference between:

- a real Flux-reconciled path (`podinfo`)
- D2 brownfield resources that carry Flux ownership labels but are not fully controller-driven
- Helm-only and native resources outside the Flux-managed path

That contrast is useful for brownfield conversations, because it shows why import and evidence need to be precise about what is controller-owned and what is only label-shaped.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
cd incubator/gitops-import-flux
./cleanup.sh
```
