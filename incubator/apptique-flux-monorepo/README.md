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
- your current Kubernetes context
- your current Flux installation in that cluster

## What It Writes

It writes live infrastructure only:

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
kubectl config current-context
flux get all -A
```

The `--explain` commands are read-only.

## Quick Start

Dev only:

```bash
cd incubator/apptique-flux-monorepo
./setup.sh
./verify.sh
```

Dev and prod:

```bash
./setup.sh --with-prod
./verify.sh --with-prod
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by applying the GitRepository and the dev Kustomization.

`./setup.sh --with-prod` mutates live infrastructure further by applying the prod Kustomization.

`./verify.sh` is read-only.

`./cleanup.sh` mutates live infrastructure by deleting the GitRepository, Kustomizations, and namespaces.

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
kubectl get gitrepositories,kustomizations -n flux-system
kubectl get deployment,service -n apptique-dev
kubectl get deployment,service -n apptique-prod
flux get sources git -A
flux get kustomizations -A
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
- [../springboot-platform-app](../springboot-platform-app/README.md)

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
