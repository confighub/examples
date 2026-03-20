# GitOps Import Argo

This incubator example is the working Argo import path for the current GitHub + Argo + AI/CLI + ConfigHub wedge.

It sets up a local kind cluster with real ArgoCD, installs a ConfigHub worker if requested, and demonstrates `cub gitops discover` and `cub gitops import` against ArgoCD `Application` resources.

It also includes an optional contrast layer adapted from `cub-scout` so you can compare two different Argo-shaped situations in one cluster:

- real ArgoCD-synced guestbook applications
- ArgoCD-labeled brownfield resources that were not created by ArgoCD

## What This Example Is For

Use this example when you want to show that ConfigHub can import GitOps-managed WET configuration from an ArgoCD environment, organize it in ConfigHub, and give you evidence to inspect without requiring ConfigHub to be the workload applier.

This is not the layered recipe story from `global-app-layer`. This is the import-and-evidence story.

## What It Reads

It reads:

- local setup scripts in `bin/`
- the kind cluster config in `cluster/`
- the sample ArgoCD GitOps repo referenced by `bin/setup-apps`
- optional contrast fixtures in `fixtures/`
- your current `cub` auth and `CUB_SPACE` if you install the worker

## What It Writes

It writes local state:

- `var/<cluster>.kubeconfig`
- `var/argocd-admin-password.txt`
- `var/argocd-host-port.txt`

It writes live infrastructure:

- a kind cluster
- ArgoCD installation in that cluster
- ArgoCD projects, applications, and application sets
- optional contrast fixtures
- optional ConfigHub worker deployment in the cluster

It writes ConfigHub state only if you choose the worker and import path:

- a worker in the selected space
- targets registered by that worker
- discover, dry, wet, and linked units created by `cub gitops discover` and `cub gitops import`

## Read-Only First

Preview the setup plan before you mutate anything:

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not create a cluster, do not create ConfigHub objects, and do not touch live infrastructure.

## Quick Start

Cluster and ArgoCD only:

```bash
cd incubator/gitops-import-argo
./setup.sh
./verify.sh
```

Cluster, ArgoCD, and optional contrast fixtures:

```bash
./setup.sh --with-contrast
./verify.sh
```

Cluster, ArgoCD, ConfigHub worker, and optional contrast fixtures:

```bash
cub auth login
export CUB_SPACE=<space>
./setup.sh --with-worker --with-contrast
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating a cluster and installing ArgoCD.

`./setup.sh --with-worker` mutates ConfigHub and live infrastructure by creating a worker and installing it into the cluster.

`cub gitops discover` mutates ConfigHub only by creating or reusing the discover unit.

`cub gitops import` mutates ConfigHub only by creating renderer units, wet units, and links.

`./cleanup.sh` mutates live infrastructure and local files by deleting the kind cluster and local kubeconfig state.

If `9080` is already in use, the example automatically picks the first free port in `9080-9099` for the ArgoCD UI and API and records it in `var/argocd-host-port.txt`. You can override that explicitly with `export ARGOCD_HOST_PORT=<port>` before running `./setup.sh`.

## Access

After setup:

- ArgoCD UI: `http://localhost:$(cat var/argocd-host-port.txt)`
- ArgoCD username: `admin`
- ArgoCD password: `cat var/argocd-admin-password.txt`
- kubeconfig: `export KUBECONFIG=$PWD/var/gitops-import-argo.kubeconfig`

## Discover And Import

If you installed the worker and have a valid `CUB_SPACE`, the worker should register two targets:

- `worker-kubernetes-yaml-cluster`
- `worker-argocdrenderer-kubernetes-yaml-cluster`

Use those for the import path:

```bash
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" worker-kubernetes-yaml-cluster --json | jq
cub gitops import --space "$CUB_SPACE" worker-kubernetes-yaml-cluster worker-argocdrenderer-kubernetes-yaml-cluster --wait
```

## What Success Looks Like

At the cluster level you should see:

- a reachable kind cluster
- ArgoCD running in the `argocd` namespace
- ArgoCD applications present in the cluster
- if `--with-contrast` was used, both guestbook and brownfield-style Argo fixtures in place

At the ConfigHub level, after discover and import, you should see:

- discovered GitOps resources from `cub gitops discover`
- `-dry` and `-wet` units in the selected space
- successful unit actions for the renderer stage
- imported WET configuration available for inspection

## Live Verification Model

For LIVE work, use a layered verification model instead of relying on one status surface.

Use direct cluster commands to prove raw runtime facts.

Use ConfigHub commands to prove import, render, and stored WET configuration facts.

Use `cub-scout` to prove live ownership and GitOps context facts, especially when the cluster contains a mix of:

- resources really created by ArgoCD
- resources that only carry ArgoCD ownership signals
- Helm-managed resources
- native unmanaged resources

That is particularly important for brownfield-style examples. It lets the example show not just that something exists, but who is really managing it.

## Evidence To Check

Direct cluster evidence:

```bash
export KUBECONFIG=$PWD/var/gitops-import-argo.kubeconfig
kubectl get applications -n argocd
kubectl get all -A
```

ConfigHub evidence:

```bash
cub unit list --space "$CUB_SPACE" --json | jq
cub unit-action list --space "$CUB_SPACE" <unit-slug>
cub unit-action get --space "$CUB_SPACE" <unit-slug> <queued-operation-id> --json | jq
```

Optional `cub-scout` live ownership and GitOps context evidence:

```bash
cub-scout gitops status
cub-scout map list
```

Important: import and renderer evidence are not the same thing as proving that a live GitOps controller reconciled workloads. If you need runtime truth, compare ConfigHub evidence and `cub-scout` evidence with direct cluster evidence.

## Interpreting ArgoCDRenderer Evidence

When the imported unit is itself an ArgoCD `Application`, the `ArgoCDRenderer` path can prove useful things, but it is still only a partial proof.

It can prove that:

- the worker processed the `Application` correctly
- the renderer target accepted it
- ArgoCD refreshed or reconciled its view of that application

It does not by itself prove that the specific ConfigHub action created new workloads. If the workloads already existed and ArgoCD was already managing them, the strongest honest conclusion is usually that ConfigHub successfully triggered or refreshed Argo-side reconciliation, not that ConfigHub created the workloads through Argo.

## Optional Contrast Path

The optional contrast fixtures are adapted from `cub-scout`. They let you inspect the difference between:

- Applications that ArgoCD really syncs
- Resources that carry ArgoCD ownership signals but were not created by ArgoCD

That contrast is useful for brownfield conversations, because it shows why import and evidence need to be precise about what is controller-owned and what is only label-shaped.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
cd incubator/gitops-import-argo
./cleanup.sh
```
