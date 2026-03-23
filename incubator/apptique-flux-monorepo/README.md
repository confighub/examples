# Apptique Flux Monorepo

This incubator example adapts the Flux monorepo app-style pattern from `cub-scout` into the official `examples` repo.

It shows one realistic GitOps app layout:

- one GitRepository
- one shared app base
- one dev overlay
- one prod overlay
- one Flux Kustomization per environment

This is an app-style example, not an import example.

## What This Example Is For

Use this example when you want to show:

- a real Flux monorepo layout for one app
- clear separation between base manifests and environment overlays
- how Flux Kustomizations map environments to namespaces
- how to verify ownership and provenance with `kubectl`, `flux`, and optionally `cub-scout`

If you want to import a live Flux cluster into ConfigHub, use [../gitops-import-flux](../gitops-import-flux/README.md) after or alongside this example.

## Source

This example is adapted from the Flux monorepo pattern in `cub-scout`:

- [cub-scout apptique flux-monorepo](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples/flux-monorepo)

## What It Reads

It reads:

- the copied app manifests under `apps/apptique/`
- the Flux GitRepository and Kustomization CRs under `infrastructure/` and `clusters/`
- a local `kind` cluster created by `setup.sh`
- a dedicated kubeconfig under `var/`

## What It Writes

It writes live infrastructure only:

- one local `kind` cluster
- Flux `source-controller` and `kustomize-controller`
- one Flux GitRepository in `flux-system`
- one Flux Kustomization for `apptique-dev`
- optionally one Flux Kustomization for `apptique-prod`
- the resulting namespaces, deployment, and service

It does not write ConfigHub state by itself.

## Read-Only First

```bash
cd incubator/apptique-flux-monorepo
./setup.sh --explain
./setup.sh --explain-json | jq
```

The `--explain` commands are read-only.

## Quick Start

Dev only:

```bash
./setup.sh
./verify.sh
```

Dev and prod:

```bash
./setup.sh --with-prod
./verify.sh --with-prod
```

## Optional Branch Override

For branch-backed validation before merge:

```bash
EXAMPLES_GIT_REVISION=<branch-name> ./setup.sh --with-prod
```

By default the example uses `main`.

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster, installing Flux, and applying the GitRepository plus the dev Kustomization.

`./setup.sh --with-prod` mutates live infrastructure further by applying the prod Kustomization.

`./verify.sh` is read-only.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and dedicated kubeconfig.

## What Success Looks Like

At the cluster level you should see:

- `GitRepository/apptique-examples` in `flux-system`
- `Kustomization/apptique-dev` ready in `flux-system`
- `Namespace/apptique-dev`
- `Deployment/frontend` available in `apptique-dev`
- `Service/frontend` in `apptique-dev`

With `--with-prod`, you should also see the matching prod objects.

At the ownership and provenance level, you should be able to trace:

- `Deployment/frontend`
- back to the environment Kustomization
- back to the GitRepository source

## Evidence To Check

Direct cluster evidence:

```bash
kubectl --kubeconfig var/apptique-flux-monorepo.kubeconfig get gitrepositories,kustomizations -n flux-system
kubectl --kubeconfig var/apptique-flux-monorepo.kubeconfig get deployment,service -n apptique-dev
kubectl --kubeconfig var/apptique-flux-monorepo.kubeconfig get deployment,service -n apptique-prod
flux --kubeconfig var/apptique-flux-monorepo.kubeconfig get sources git -A
flux --kubeconfig var/apptique-flux-monorepo.kubeconfig get kustomizations -A
```

Optional `cub-scout` evidence:

```bash
cub-scout map list -q "namespace=apptique-*"
cub-scout trace deployment/frontend -n apptique-dev
cub-scout gitops status
```

This is the same evidence-first model as the import examples:

- `kubectl` and `flux` prove raw cluster facts
- `cub-scout` proves ownership and GitOps context facts

## Why This Example Matters

This gives the incubator set one clear app-style example that is not mainly about import.

It shows a realistic app layout teams actually use:

- one app
- one base
- multiple overlays
- separate environment Kustomizations

That makes it a good companion to:

- [../gitops-import-flux](../gitops-import-flux/README.md)
- [../gitops-import-argo](../gitops-import-argo/README.md)
- [../apptique-argo-applicationset](../apptique-argo-applicationset/README.md)

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
