# e2e: Global App Layer Lifecycle Tests

This directory now contains the full end-to-end test layer for the `global-app-layer` package.

It combines two things that belong together:

- full lifecycle tests across brownfield, greenfield, and bridge flows
- shared delivery helpers for applying a single example directly or via ArgoCD

## What This Test Layer Is For

Use this when the reason for the work is lifecycle proof, not example onboarding.

This directory exists to exercise the package as a system: brownfield import, greenfield build-up, bridge flows, and delivery helpers that show what a real end-to-end test of `global-app-layer` should look like.

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

Deliver one already-materialized example using the current Argo-oriented e2e path.

```bash
./e2e/deliver-argo.sh frontend-postgres
./e2e/assert-cluster.sh frontend-postgres
```

Important:
- this helper is currently a hybrid path, not a pure GitOps proof
- it exports ConfigHub-rendered YAMLs, applies them directly with `kubectl`, and then creates an ArgoCD Application for visibility and drift detection
- it proves the staged Argo-shaped workflow and Argo artifacts around the app
- it does not, by itself, prove Argo reconciled the workloads from Git as the sole delivery mechanism
- the current `ArgoCDRenderer` target is renderer-oriented as well, so it is also not the final proof of Argo-managed workload sync

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

Use those infrastructure checks as part of the demo, not as hidden setup:

- show that the worker is actually ready before delivery
- show which target type is being exercised
- for delegated delivery, show the agent-side objects and sync state, not only the final cluster resources

## Why This Lives Here

These tests are specific to the `global-app-layer` package.

They prove that the layered recipe model works:

- from imported brownfield state
- from fresh greenfield creation
- and across direct vs Argo-oriented hybrid delivery
