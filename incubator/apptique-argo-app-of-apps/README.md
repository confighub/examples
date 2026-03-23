# Apptique Argo App Of Apps

This incubator example adapts the Argo CD app-of-apps pattern from `cub-scout` into the official `examples` repo.

It shows one realistic Argo hierarchy:

- one root Application
- one child Application per environment
- one workload tree per child Application

This is an app-style example, not an import example.

## What This Example Is For

Use this example when you want to show:

- a real Argo app-of-apps hierarchy
- how one root Application manages child Applications
- how child Applications map to namespaces and workloads
- how to verify ownership and provenance with `kubectl` and optional `cub-scout`

If you want to import a live Argo cluster into ConfigHub, use [../gitops-import-argo](../gitops-import-argo/README.md) after or alongside this example.

## Source

This example is adapted from the Argo app-of-apps pattern in `cub-scout`:

- [cub-scout apptique argo-app-of-apps](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples/argo-app-of-apps)

## What It Reads

It reads:

- the copied root, child Application, and workload manifests in this directory
- a local `kind` cluster created by `setup.sh`
- a dedicated kubeconfig under `var/`

## What It Writes

It writes live infrastructure only:

- one local `kind` cluster
- Argo CD in `argocd`
- one root Argo Application in `argocd`
- child Applications for `apptique-dev` and `apptique-prod`
- the resulting namespaces, deployment, and service

It does not write ConfigHub state by itself.

## Read-Only First

```bash
cd incubator/apptique-argo-app-of-apps
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

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster, installing Argo CD, applying the root Application, and letting that root app create the child Applications.

When `EXAMPLES_GIT_REVISION` is not `main`, `setup.sh` also patches the child Applications to sync that branch so pre-merge validation can exercise the branch content honestly.

`./verify.sh` is read-only.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and dedicated kubeconfig.

## What Success Looks Like

At the cluster level you should see:

- `Application/apptique-apps` in `argocd`
- child `Application/apptique-dev` and `Application/apptique-prod`
- `Namespace/apptique-dev` and `Namespace/apptique-prod`
- `Deployment/frontend` available in both namespaces
- `Service/frontend` in both namespaces

At the ownership and provenance level, you should be able to trace:

- `Deployment/frontend`
- back to the child Argo Application
- back to the root Argo Application

## Evidence To Check

Direct cluster evidence:

```bash
kubectl --kubeconfig var/apptique-argo-app-of-apps.kubeconfig get applications -n argocd
kubectl --kubeconfig var/apptique-argo-app-of-apps.kubeconfig get deployment,service -n apptique-dev
kubectl --kubeconfig var/apptique-argo-app-of-apps.kubeconfig get deployment,service -n apptique-prod
```

Optional `cub-scout` evidence:

```bash
cub-scout map list -q "owner=ArgoCD"
cub-scout trace deployment/frontend -n apptique-dev
cub-scout gitops status
```

This is the same evidence-first model as the other app-style examples:

- `kubectl` proves raw Argo and workload facts
- `cub-scout` proves ownership and provenance

## Why This Example Matters

This gives the incubator set the second major Argo app-style pattern to pair with ApplicationSet.

It shows a realistic Argo layout teams actually use:

- one root app
- one child app per environment
- one hierarchy that is explicit instead of generator-driven

That makes it a good companion to:

- [../apptique-argo-applicationset](../apptique-argo-applicationset/README.md)
- [../apptique-flux-monorepo](../apptique-flux-monorepo/README.md)
- [../gitops-import-argo](../gitops-import-argo/README.md)

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
