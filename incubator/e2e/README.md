# End-to-End Test Flows

Three test scripts that prove ConfigHub works across the full lifecycle — from importing existing cluster apps to deploying fresh layered recipes to bridging between the two.

All three require a running kind cluster with ArgoCD and a ConfigHub worker. See [gitops-import](../../gitops-import/) for setup.

## The Three Flows

### 1. Brownfield (`01-brownfield.sh`)

**Starting point:** an existing app already running on the cluster via ArgoCD.

```
existing ArgoCD app → cub gitops discover → cub gitops import
    → mutate (set-replicas) → cub unit apply → verify on cluster
```

Proves that ConfigHub can ingest a running app and immediately start managing it. The import is a one-time operation; after that, mutations and applies go through ConfigHub.

### 2. Greenfield (`02-greenfield.sh`)

**Starting point:** raw YAML files, no existing cluster state.

```
base YAML → ConfigHub layered chain (setup.sh)
    → verify chain (verify.sh) → cub unit apply → verify on cluster → cleanup
```

Runs each [global-app-layer](../global-app-layer/) example (single-component, frontend-postgres, realistic-app, gpu-eks-h100-training) through the full cycle. Override which examples run with `E2E_EXAMPLES="single-component realistic-app"`.

### 3. Bridge (`03-bridge.sh`)

**Starting point:** an existing app, then layered config on top.

```
existing ArgoCD app → import → clone into region layer
    → clone into role layer → mutate layers → apply → verify
```

This is the most realistic user journey: "I have a running app. I bring it into ConfigHub. Then I start layering region and role config on top of what I imported." Proves that brownfield and greenfield are not separate worlds — you can import and immediately start layering.

## Running

```bash
# All three flows
./incubator/e2e/run-all.sh

# Individual flows
./incubator/e2e/run-all.sh brownfield
./incubator/e2e/run-all.sh greenfield
./incubator/e2e/run-all.sh bridge

# Or directly
./incubator/e2e/01-brownfield.sh
./incubator/e2e/02-greenfield.sh
./incubator/e2e/03-bridge.sh
```

## Prerequisites

1. A running kind cluster: `gitops-import/bin/create-cluster`
2. ArgoCD installed: `gitops-import/bin/install-argocd`
3. Sample apps deployed: `gitops-import/bin/setup-apps`
4. ConfigHub worker registered: `CUB_SPACE=<space> gitops-import/bin/install-worker`

The brownfield and bridge flows need existing ArgoCD apps (from step 3). The greenfield flow only needs the worker.

## Shared Helpers (`lib.sh`)

The `lib.sh` file provides:

- `require_infrastructure` — checks kubeconfig, cluster reachability, and worker readiness
- `direct_target` / `argo_target` — returns the target ref for direct or ArgoCD delivery
- `ensure_namespace` — creates a namespace if it doesn't exist
- `wait_for_deployment` / `wait_for_statefulset` / `wait_for_daemonset` — kubectl wait wrappers
- `assert_resource_exists` / `assert_resource_gone` / `assert_resource_field` — cluster assertions
- `assert_unit_exists` — ConfigHub unit assertion
- `clean_space_contents` — deletes all links and units from a space (useful for re-running imports)

## Relationship to global-app-layer/e2e/

The [global-app-layer/e2e/](../global-app-layer/e2e/) directory contains **delivery scripts** (`deliver-direct.sh`, `deliver-argo.sh`) that are specific to applying a single global-app-layer example to a cluster. They are the "deliver" phase of a single example.

This directory contains **full lifecycle tests** that orchestrate setup → deliver → assert → cleanup across multiple examples and import flows.
