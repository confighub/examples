# Start Here: ConfigHub Examples Without Extra Complexity

If ConfigHub feels overwhelming, use this flow first.

## One Command Verification

```bash
./scripts/verify.sh
```

This checks:

- wrapper script syntax
- bundle shape (`up.yaml` + manifests)
- `cub-up` bundle directories

## Mental Model

ConfigHub is the decision and governance plane. Workers execute. External controllers (Argo, Flux) reconcile.

```
Flux/Argo reconcile. ConfigHub workers execute. ConfigHub governs.
```

### Core concepts

- **Unit**: desired configuration — the manifests or controller intent you want applied.
- **Worker**: a ConfigHub-managed bridge process that executes operations against a target. Workers can delegate to controller-based workflows (e.g., applying an Argo `Application` CR).
- **Target**: a worker-bound endpoint defined by toolchain and provider. Not limited to Kubernetes — current providers include Kubernetes, Flux/Argo variants, OpenTofu, AWS, and others.
- **Assertions**: explicit state checks after each major command.

### Wiring

```
unit → worker (bridge) → target
```

For controller-delegated flows (e.g., ArgoCD), the worker applies a controller CR to the target, then watches sync/health status. The controller handles actual reconciliation.

### `app` vs `platform` (cub-up bundle convention)

In current `cub-up` examples, the `kind` field in `up.yaml` distinguishes two apply paths:

- **`app`**: unit contains direct workload manifests (Deployment, Service). Worker applies them to the target.
- **`platform`**: unit contains controller intent (e.g., Argo `Application`). Worker applies the CR; the controller reconciles the actual workload.

### When to create new resources

- **New target**: when you need a different endpoint (different cluster, account, or provider).
- **New unit**: when you have a distinct piece of configuration to manage independently.
- **New app/platform bundle**: when you want a self-contained `cub-up` scenario with its own units, environment, and target binding.
- **Reuse existing**: when iterating on the same workload in the same destination. Use `--on-exists reuse` or `prompt` to decide interactively.

## Run Modes

Choose one contract and keep it explicit:

- Human-led: `./scripts/cub-up-human-flow.sh ...`
- AI-led: `./scripts/cub-up-ai-flow.sh ...`
- Human+AI pair: `./scripts/cub-up-pair-flow.sh ...`

## First Demo Command

```bash
CUB_UP_ON_EXISTS=fresh CUB_UP_STALE_ACTION=fresh \
./scripts/cub-up-ai-flow.sh app ./incubator/cub-up/global-app dev <existing-target>
```

Why this default:

- avoids stale/reused slugs in demos
- keeps assertions and GUI checkpoints visible
- reduces confusion about old/disconnected state
