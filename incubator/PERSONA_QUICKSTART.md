# Persona Quickstart: Jesper, Ilya, Claude

Use this when you want the shortest path to the right current incubator example.

## Shared Prerequisites

```bash
cub auth login
./scripts/verify.sh
```

## Jesper

Goal: show a believable app-level recipe story that still stays small enough to review.

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh
./verify.sh
```

## Ilya

Goal: show where layered, reproducible infrastructure recipes could go next.

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh
./verify.sh
```

## Claude

Goal: learn the model from the smallest example, then move to the realistic app.

```bash
cd incubator/global-app-layer/single-component
./setup.sh
./verify.sh

cd ../realistic-app
./setup.sh
./verify.sh
```

## Tiny fixtures

If you only want the smallest direct and delegated apply inputs for `cub run` design work:

```bash
cd incubator/cub-run-fixtures
ls
```
