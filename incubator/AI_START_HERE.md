# AI Start Here

Use this page as the single AI-oriented handoff page for the current incubator work.

## Goal

Start with examples that are real, current, and easy to verify.

## 0) Prerequisites

```bash
cd <your-examples-checkout>
cub auth login
```

If you want to run against a live target, have one ready in your active space.

Quick check:

```bash
cub target list --no-header
```

## 1) Recommended first path

Start with the realistic layered app example:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh
./verify.sh
```

If you want to wire a real target immediately:

```bash
./setup.sh <prefix> <space/target>
./verify.sh
```

## 2) Quick demo data (no cluster required)

For exploring ConfigHub's promotion UI without a live target:

```bash
cd promotion-demo-data
./setup.sh
./cleanup.sh
```

For CI/AI verification, see [promotion-demo-data-verify](./promotion-demo-data-verify/).

This creates 49 spaces and ~154 units using the **App-Deployment-Target** model:

- **App** → label + units (e.g., `aichat`, `eshop`, `platform`)
- **Target** → infra space with target object (e.g., `us-prod-1`)
- **Deployment** → `{target}-{app}` space (e.g., `us-prod-1-eshop`)

Uses the noop bridge, so no Kubernetes cluster is needed. This is the canonical multi-env model for ConfigHub.

## 3) Smaller and larger options

Smallest:

```bash
cd incubator/global-app-layer/single-component
./setup.sh
./verify.sh
```

GPU-flavored:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh
./verify.sh
```

## 4) What success looks like

You should be able to see:

- explicit spaces and units created
- clone-chain structure preserved
- recipe manifest materialized
- verification passing against the created ConfigHub objects

## 5) Tiny direct vs delegated fixtures

If you need the smallest possible direct and delegated apply inputs for design work around `cub run`, use:

- [cub-run-fixtures](./cub-run-fixtures/README.md)

These are preserved reference fixtures, not the main walkthrough.

## 6) Related Pages

- Start guide: [START_HERE.md](./START_HERE.md)
- Current package: [global-app-layer/README.md](./global-app-layer/README.md)
