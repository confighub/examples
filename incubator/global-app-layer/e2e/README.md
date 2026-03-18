# e2e: Global App Layer Lifecycle Tests

This directory now contains the full end-to-end test layer for the `global-app-layer` package.

It combines two things that belong together:

- full lifecycle tests across brownfield, greenfield, and bridge flows
- shared delivery helpers for applying a single example directly or via ArgoCD

## The Three Lifecycle Flows

### 1. Brownfield (`01-brownfield.sh`)

Start with an app that already exists on the cluster via ArgoCD.

```text
existing ArgoCD app
  -> cub gitops discover
  -> cub gitops import
  -> mutate in ConfigHub
  -> cub unit apply
  -> verify on cluster
```

### 2. Greenfield (`02-greenfield.sh`)

Start with raw manifests and build the full layered chain in ConfigHub.

```text
base YAML
  -> setup.sh
  -> verify.sh
  -> cub unit apply
  -> verify on cluster
  -> cleanup
```

### 3. Bridge (`03-bridge.sh`)

Import an existing app and then start layering on top of it.

```text
existing ArgoCD app
  -> import into ConfigHub
  -> clone into region layer
  -> clone into role layer
  -> mutate layers
  -> apply
  -> verify on cluster
```

## Example Delivery Helpers

### `deliver-direct.sh`

Apply the deployment units for one already-materialized example directly through the ConfigHub worker.

```bash
./e2e/deliver-direct.sh frontend-postgres
./e2e/assert-cluster.sh frontend-postgres
```

### `deliver-argo.sh`

Deliver one already-materialized example using the Argo-oriented path.

```bash
./e2e/deliver-argo.sh frontend-postgres
./e2e/assert-cluster.sh frontend-postgres
```

## Running the Lifecycle Flows

```bash
# All three flows
./incubator/global-app-layer/e2e/run-all.sh

# Individual flows
./incubator/global-app-layer/e2e/run-all.sh brownfield
./incubator/global-app-layer/e2e/run-all.sh greenfield
./incubator/global-app-layer/e2e/run-all.sh bridge

# Or directly
./incubator/global-app-layer/e2e/01-brownfield.sh
./incubator/global-app-layer/e2e/02-greenfield.sh
./incubator/global-app-layer/e2e/03-bridge.sh
```

## Prerequisites

1. A running kind cluster: `gitops-import/bin/create-cluster`
2. ArgoCD installed: `gitops-import/bin/install-argocd`
3. Sample apps deployed: `gitops-import/bin/setup-apps`
4. ConfigHub worker registered: `CUB_SPACE=<space> gitops-import/bin/install-worker`

The brownfield and bridge flows need existing ArgoCD apps. The greenfield flow only needs the worker.

## Shared Helpers

The shared `lib.sh` provides:

- infrastructure checks
- target refs for direct and Argo delivery
- example loading from `.state/state.env`
- cluster assertions and wait helpers
- ConfigHub space cleanup helpers

## Why This Lives Here

These tests are specific to the `global-app-layer` package.

They prove that the layered recipe model works:

- from imported brownfield state
- from fresh greenfield creation
- and across direct vs delegated delivery
