# Why ConfigHub

ConfigHub is a management layer for operational configuration.

It sits between your config sources (Git, Helm charts, existing clusters) and your deployment targets (Kubernetes, Flux, Argo CD). It stores versioned config, applies policy, tracks provenance, and executes controlled delivery.

## What ConfigHub Adds Beyond GitOps

GitOps controllers (Argo CD, Flux) are powerful reconcilers. They sync Git state to cluster state.

ConfigHub adds:

- **Unified view**: import and compare config from Git, live clusters, and multiple controllers in one place
- **Policy and scanning**: validate config against rules before delivery, not after
- **Mutation control**: make controlled changes through one write API with audit
- **Layered composition**: build deployment variants from shared recipes with tracked ancestry
- **Delivery options**: apply through direct workers, Flux OCI, or Argo OCI depending on your environment

ConfigHub does not replace your GitOps controller. It extends what you can see and what you can safely change.

## The Four Reasons to Use ConfigHub

### Import

Ingest existing config from Git repos, live clusters, or GitOps controllers. Compare what Git says with what the cluster has. Surface drift, ownership gaps, and orphaned resources.

**Start here**: [gitops-import-argo](./gitops-import-argo/README.md) or [gitops-import-flux](./gitops-import-flux/README.md)

### Mutate

Use ConfigHub as a write API for operational config. Apply governed changes through explicit functions (`set-replicas`, `set-env`, `set-namespace`) with audit and approval workflows.

**Start here**: [platform-write-api](./platform-write-api/README.md)

### Apply

Deliver real changes to real targets. ConfigHub stores the intended state, renders it for the target, and applies through workers or controllers with verification.

**Start here**: [springboot-platform-app](./springboot-platform-app/README.md) or [global-app-layer/single-component](./global-app-layer/single-component/README.md)

### Model

Represent layered or governed configuration structures. Build recipes from shared components, track variant ancestry, preserve provenance across deployment stages.

**Start here**: [global-app-layer](./global-app-layer/README.md) or [global-app-layer/gpu-eks-h100-training](./global-app-layer/gpu-eks-h100-training/README.md)

## Delivery Matrix

ConfigHub supports multiple delivery modes. The right choice depends on your environment:

| Delivery Mode | What It Does | When To Use |
|---------------|--------------|-------------|
| **Direct Kubernetes** | Worker applies YAML directly via `kubectl apply` | Simplest real proof. No controller required. |
| **Flux OCI** | Worker publishes to ConfigHub-native OCI origin, Flux reconciles workloads from it | Standard controller path today. Flux manages workload lifecycle. External registries are optional. |
| **Argo OCI** | Worker publishes to ConfigHub-native OCI origin, Argo reconciles from it | Implemented. Claim it only when controller and live evidence are shown. |
| **Renderer-only** | Worker sends payloads to a renderer (e.g., `ArgoCDRenderer`) for hydration | Companion path for rendering. Not the same as OCI delivery. |

For most examples in this repo:

- **Direct Kubernetes** is the simplest fully proven path
- **Flux OCI** is the current standard for controller-oriented delivery
- **Argo OCI** is implemented, but it still needs controller and live evidence when claimed
- **ArgoCDRenderer** is a valid renderer path but should not be confused with Argo OCI delivery

## Incubator Glossary

| Term | Meaning |
|------|---------|
| **Space** | A named workspace in ConfigHub. Each layer or deployment stage typically gets its own space. |
| **Unit** | One versioned config object. Usually one manifest or manifest set. |
| **Variant** | A specialized unit derived from an earlier unit. The user-facing idea of a layered copy. |
| **Clone link** | The ConfigHub mechanism behind a variant. How shared changes flow downstream with provenance. |
| **Worker** | A long-lived agent that registers delivery endpoints and executes apply/import work. |
| **Target** | A named delivery endpoint owned by a worker. Where ConfigHub applies or publishes rendered config. |
| **Bundle** | The deployable output produced for a target. Bundles belong to targets, not to recipes. |
| **Recipe manifest** | A metadata unit that explains how a recipe was assembled. The receipt, not another layer. |
| **Deployment variant** | The final target-specific form of a unit, ready for delivery. |
| **WET** | "Write Everything Twice" — explicit, materialized config rather than templated or generated. |
| **Brownfield** | Importing from an existing system rather than starting fresh. |

## Quick Decision Tree

```
What do you want to do?
│
├─ See what I already have → Import
│   └─ gitops-import-argo or gitops-import-flux
│
├─ Make controlled changes → Mutate
│   └─ platform-write-api or springboot-platform-app
│
├─ Deploy real workloads → Apply
│   └─ springboot-platform-app or global-app-layer/single-component
│
└─ Model layered config → Model
    └─ global-app-layer or gpu-eks-h100-training
```

## Next Steps

- For the standard GitOps import story, start with [gitops-import-argo](./gitops-import-argo/README.md) or [gitops-import-flux](./gitops-import-flux/README.md)
- For the smallest real apply proof, start with [global-app-layer/single-component](./global-app-layer/single-component/README.md)
- For the full incubator overview, see [README.md](./README.md)
- For AI-oriented guidance, see [AI_START_HERE.md](./AI_START_HERE.md)
