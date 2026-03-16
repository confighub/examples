# Start Here

Use this page to choose the right incubator example without getting lost in old paths.

## One Command Verification

```bash
./scripts/verify.sh
```

This checks:

- incubator shell script syntax
- recipe example script syntax
- tiny fixture layout under `incubator/cub-run-fixtures`

## Mental Model

ConfigHub is the decision and governance plane. Workers execute. External controllers such as Argo and Flux reconcile when the apply mode is delegated.

```
ConfigHub governs. Workers execute. External controllers reconcile.
```

### Core concepts

- **Unit**: desired configuration text.
- **Target**: where apply or destroy runs.
- **Worker**: process connected to target operations.

### Wiring

```
unit → worker (bridge) → target
```

For delegated flows such as ArgoCD, the worker applies a controller CR to the target, then reconciliation and health are observed through that controller.

### Apply modes

When the execution path matters, prefer this language:

- **`apply: direct`**: worker applies workload manifests to the target.
- **`apply: argo`**: worker applies an Argo `Application`; Argo reconciles the workload later.
- **`apply: flux`**: worker publishes or applies Flux-managed intent; Flux reconciles later.

## Recommended Entry Points

### Smallest worked recipe example

```bash
cd incubator/global-app-layer/single-component
./setup.sh
./verify.sh
```

### Most realistic current app example

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh
./verify.sh
```

### GPU-flavored layered example

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh
./verify.sh
```

### Tiny apply-mode fixtures

If you only want the smallest direct-vs-delegated reference inputs, read:

- [cub-run-fixtures](./cub-run-fixtures/README.md)

These fixtures are preserved as design seeds, not as the main demo path.

## Which Path Fits Best

### Jesper

If you want the most believable small app story first:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh
./verify.sh
```

### Ilya

If you want the clearest route from current examples toward layered infrastructure recipes:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh
./verify.sh
```

### Claude or another AI agent

If you want the smallest learning path first, then the more realistic app:

```bash
cd incubator/global-app-layer/single-component
./setup.sh
./verify.sh

cd ../realistic-app
./setup.sh
./verify.sh
```

If you want a single AI-oriented handoff page instead of this overview, use:

- [AI_START_HERE.md](./AI_START_HERE.md)
