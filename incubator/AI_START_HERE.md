# AI Start Here

Use this page as the single AI-oriented handoff page for the current incubator work.

## Goal

Start with examples that are real, current, and easy to verify.

## 0) Prerequisites

```bash
cd <your-examples-checkout>
cub auth login
./scripts/verify.sh
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

## 2) Smaller and larger options

Smallest:

```bash
cd ../single-component
./setup.sh
./verify.sh
```

GPU-flavored:

```bash
cd ../gpu-eks-h100-training
./setup.sh
./verify.sh
```

## 3) What success looks like

You should be able to see:

- explicit spaces and units created
- clone-chain structure preserved
- recipe manifest materialized
- verification passing against the created ConfigHub objects

## 4) Tiny direct vs delegated fixtures

If you need the smallest possible direct and delegated apply inputs for design work around `cub run`, use:

- [cub-run-fixtures](./cub-run-fixtures/README.md)

These are preserved reference fixtures, not the main walkthrough.

## 5) Related Pages

- Start guide: [START_HERE.md](./START_HERE.md)
- Current package: [global-app-layer/README.md](./global-app-layer/README.md)
