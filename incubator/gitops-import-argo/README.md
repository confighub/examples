# GitOps Import Argo

This incubator example is the working Argo import path for the current GitHub + Argo + AI/CLI + ConfigHub wedge.

It sets up a local kind cluster with real ArgoCD, installs a ConfigHub worker if requested, and demonstrates `cub gitops discover` and `cub gitops import` against ArgoCD `Application` resources.

It also includes an optional contrast layer adapted from `cub-scout` so you can compare two different Argo-shaped situations in one cluster:

- real ArgoCD-synced guestbook applications
- ArgoCD-labeled brownfield resources that were not created by ArgoCD

## What This Example Is For

Use this example when you want to show that ConfigHub can import GitOps-managed WET configuration from an ArgoCD environment, organize it in ConfigHub, and give you evidence to inspect without requiring ConfigHub to be the workload applier.

This is not the layered recipe story from `global-app-layer`. This is the import-and-evidence story.

## Standard Story

This is the standard Argo story in the incubator.

Lead with the healthy guestbook applications first. They are the front door for showing a real ArgoCD environment, direct cluster evidence, and then ConfigHub discover/import against the same controller-owned objects.

Do not lead with `--with-contrast`. The brownfield contrast fixtures are useful, but they are second-pass material after the guestbook path has already created value.

## What It Reads

It reads:

- local setup scripts in `bin/`
- the kind cluster config in `cluster/`
- the local guestbook Application fixture in `fixtures/argocd/`
- the upstream `argocd-example-apps` repo referenced by those guestbook Applications
- optional contrast fixtures in `fixtures/`
- your current `cub` auth and `CUB_SPACE` if you install the worker

## What It Writes

It writes local state:

- `var/<cluster>.kubeconfig`
- `var/argocd-admin-password.txt`
- `var/argocd-host-port.txt`
- `var/worker.pid`
- `var/worker.log`
- `var/argocd-port-forward.pid`
- `var/argocd-port-forward.log`

It writes live infrastructure:

- a kind cluster
- ArgoCD installation in that cluster
- healthy guestbook ArgoCD Applications by default
- optional contrast fixtures

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

Standard first pass: healthy Argo path only.

```bash
cd incubator/gitops-import-argo
./setup.sh
./verify.sh
```

If you want the ConfigHub import path in the same session:

```bash
cub auth login
export CUB_SPACE=<space>
./setup.sh --with-worker
./verify.sh
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" worker-kubernetes-yaml-cluster --json | jq
cub target update --space "$CUB_SPACE" --patch --option IsAuthoritative=true \
  worker-argocdrenderer-kubernetes-yaml-cluster
cub gitops import --space "$CUB_SPACE" worker-kubernetes-yaml-cluster worker-argocdrenderer-kubernetes-yaml-cluster --wait
```

Only after the standard path lands, add optional contrast fixtures:

```bash
./setup.sh --with-contrast
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating a cluster, installing ArgoCD, and applying the healthy guestbook Applications.

`./setup.sh --with-worker` mutates ConfigHub and local process state by creating a worker, starting a local ArgoCD API port-forward, generating an ArgoCD API token, and starting a local Kubernetes plus ArgoCD renderer worker.

`cub gitops discover` mutates ConfigHub only by creating or reusing the discover unit.

`cub gitops import` mutates ConfigHub only by creating renderer units, wet units, and links.

`./cleanup.sh` mutates live infrastructure and local files by deleting the kind cluster, stopping the local worker if present, and removing local kubeconfig state.

If `9080` is already in use, the example automatically picks the first free port in `9080-9099` for the ArgoCD UI and API and records it in `var/argocd-host-port.txt`. You can override that explicitly with `export ARGOCD_HOST_PORT=<port>` before running `./setup.sh`.

## Access

After setup:

- ArgoCD UI: `https://localhost:$(cat var/argocd-host-port.txt)`
- ArgoCD username: `admin`
- ArgoCD password: `cat var/argocd-admin-password.txt`
- kubeconfig: `export KUBECONFIG=$PWD/var/gitops-import-argo.kubeconfig`

Expect a self-signed certificate warning in the browser. That is normal for this local demo.

## Discover And Import

If you installed the worker and have a valid `CUB_SPACE`, the worker should register two targets:

- `worker-kubernetes-yaml-cluster`
- `worker-argocdrenderer-kubernetes-yaml-cluster`

Use those for the import path:

```bash
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" worker-kubernetes-yaml-cluster --json | jq
cub target update --space "$CUB_SPACE" --patch --option IsAuthoritative=true \
  worker-argocdrenderer-kubernetes-yaml-cluster
cub gitops import --space "$CUB_SPACE" worker-kubernetes-yaml-cluster worker-argocdrenderer-kubernetes-yaml-cluster --wait
```

This authoritative step matters now. If the source Argo `Application` is still
actively syncing and the renderer target is unset or non-authoritative, import
should now fail visibly. Treat that as a correct protection, not a flaky
warning.

The worker runs locally and records its pid and log in `var/worker.pid` and `var/worker.log`. The helper also keeps a local ArgoCD API port-forward for the renderer worker in `var/argocd-port-forward.pid` and `var/argocd-port-forward.log`.

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

In the current contrast setup, the most useful live outcome is mixed on purpose:

- `helm-guestbook` and `kustomize-guestbook` are the healthy reference applications
- if `--with-contrast` is used, the brownfield fixtures add extra Argo-shaped objects that should be inspected separately from the healthy guestbook path

That gives the example immediate value: the import path shows which Applications render cleanly and which ones already have live controller-side problems.

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

## Known Live Outcome

On the fresh standard-path run executed on March 27, 2026:

- `./setup.sh` completed in about 157 seconds on a clean local kind cluster
- `helm-guestbook` and `kustomize-guestbook` both reached `Synced` and `Healthy`
- the guestbook workloads in the `guestbook` namespace were running and available
- `cub-scout gitops status` reported both deployers healthy

That is the current front-door proof. It shows a healthy real ArgoCD environment quickly, before you add the optional brownfield contrast path or the ConfigHub worker/import path.

## Interpreting ArgoCDRenderer Evidence

> **Important**: `ArgoCDRenderer` is **not** Argo OCI delivery. It is a renderer path that expects Argo `Application` CRD payloads. For native Argo OCI delivery (where ConfigHub publishes OCI artifacts that Argo consumes), see `global-app-layer/single-component` or `global-app-layer/gpu-eks-h100-training` with an `ArgoCDOCI` target.

When the imported unit is itself an ArgoCD `Application`, the `ArgoCDRenderer` path can prove useful things, but it is still only a partial proof.

It can prove that:

- the worker processed the `Application` correctly
- the renderer target accepted it
- ArgoCD refreshed or reconciled its view of that application

For current renderer behavior, that statement assumes the renderer target is
authoritative or the source `Application` is no longer actively syncing. A
non-authoritative renderer pointed at an actively syncing source should now
fail instead of only warning.

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
