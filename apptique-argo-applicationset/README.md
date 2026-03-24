# Apptique Argo ApplicationSet

This stable example adapts the Argo CD ApplicationSet app-style pattern from `cub-scout` into the official `examples` repo.

It shows one realistic Argo GitOps layout:

- one ApplicationSet
- one app directory tree
- one generated Application per environment
- one namespace per environment

This is an app-style example, not an import example.

## What This Example Is For

Use this example when you want to show:

- a real Argo ApplicationSet pattern for environment discovery
- how one generator turns directory structure into Applications
- how generated Applications map to namespaces and workloads
- how to verify ownership and provenance with `kubectl` and optionally `cub-scout`

If you want to import a live Argo cluster into ConfigHub, use [../incubator/gitops-import-argo](../incubator/gitops-import-argo/README.md) after or alongside this example.

## Source

This example is adapted from the Argo ApplicationSet pattern in `cub-scout`:

- [cub-scout apptique argo-applicationset](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples/argo-applicationset)

## What It Reads

It reads:

- the copied ApplicationSet and app manifests in this directory
- a local `kind` cluster created by `setup.sh`
- a dedicated kubeconfig under `var/`

## What It Writes

It writes live infrastructure only:

- one local `kind` cluster
- Argo CD in `argocd`
- one Argo CD ApplicationSet in `argocd`
- generated Applications for `apptique-dev` and `apptique-prod`
- the resulting namespaces, deployment, and service

It does not write ConfigHub state by itself.

## Read-Only First

```bash
cd apptique-argo-applicationset
./setup.sh --explain
./setup.sh --explain-json | jq
```

The `--explain` commands are read-only.

## Quick Start

```bash
./setup.sh
./verify.sh
```

## Optional Branch Override

For branch-backed validation before merge:

```bash
EXAMPLES_GIT_REVISION=<branch-name> ./setup.sh
```

By default the example uses `main`.

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster, installing Argo CD, and applying the ApplicationSet.

`./verify.sh` is read-only.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and dedicated kubeconfig.

## What Success Looks Like

At the cluster level you should see:

- `ApplicationSet/apptique` in `argocd`
- generated `Application/apptique-dev` and `Application/apptique-prod`
- `Namespace/apptique-dev` and `Namespace/apptique-prod`
- `Deployment/frontend` available in both namespaces
- `Service/frontend` in both namespaces

At the ownership and provenance level, you should be able to trace:

- `Deployment/frontend`
- back to the generated Argo Application
- back to the ApplicationSet source tree

## Evidence To Check

Direct cluster evidence:

```bash
kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get applicationsets,applications -n argocd
kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get deployment,service -n apptique-dev
kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get deployment,service -n apptique-prod
```

Optional `cub-scout` evidence:

```bash
cub-scout map list -q "owner=ArgoCD"
cub-scout trace deployment/frontend -n apptique-dev
cub-scout gitops status
```

This is the same evidence-first model as the import examples:

- `kubectl` proves raw Argo and workload facts
- `cub-scout` proves ownership and provenance

## Why This Example Matters

This gives the stable example set one clear Argo app-style example to pair with the Flux monorepo example.

It shows a realistic Argo layout teams actually use:

- one generator
- directory-based environment discovery
- one Application per environment

That makes it a good companion to:

- [../apptique-flux-monorepo](../apptique-flux-monorepo/README.md)
- [../incubator/gitops-import-argo](../incubator/gitops-import-argo/README.md)
- [../incubator/gitops-import-flux](../incubator/gitops-import-flux/README.md)

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
