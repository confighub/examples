# Examples Incubator

Experimental ConfigHub examples before promotion to stable.

## Quick Start

| You want to... | Start here |
|----------------|------------|
| Import from Argo | [gitops-import-argo](./gitops-import-argo/README.md) |
| Import from Flux | [gitops-import-flux](./gitops-import-flux/README.md) |
| See the write API story | [platform-write-api](./platform-write-api/README.md) |
| Deploy a real app | [springboot-platform-app-centric](../spring-platform/springboot-platform-app-centric/README.md) |
| Understand layered recipes | [global-app-layer/single-component](./global-app-layer/single-component/README.md) |

## Entry Paths

- For humans: [`../START_HERE.md`](../START_HERE.md)
- For AI assistants: [`AI_START_HERE.md`](./AI_START_HERE.md)
- Why ConfigHub: [`WHY_CONFIGHUB.md`](./WHY_CONFIGHUB.md)

## Example Catalog

### Import and Evidence

| Example | What it proves |
|---------|----------------|
| [gitops-import-argo](./gitops-import-argo/README.md) | Argo import from GitHub into ConfigHub |
| [gitops-import-flux](./gitops-import-flux/README.md) | Flux import from GitHub into ConfigHub |
| [combined-git-live](./combined-git-live/README.md) | Git vs live cluster alignment |
| [custom-ownership-detectors](./custom-ownership-detectors/README.md) | Platform-team ownership detection |
| [orphans](./orphans/README.md) | Unmanaged resource discovery |
| [watch-webhook](./watch-webhook/README.md) | Event streaming to webhook |

### Mutation and Write API

| Example | What it proves |
|---------|----------------|
| [platform-write-api](./platform-write-api/README.md) | ConfigHub as mutation plane |
| [springboot-platform-app-centric](../spring-platform/springboot-platform-app-centric/README.md) | Real app with three mutation routes |

### Layered Recipes

| Example | What it proves |
|---------|----------------|
| [global-app-layer/single-component](./global-app-layer/single-component/README.md) | Smallest layered recipe |
| [global-app-layer/frontend-postgres](./global-app-layer/frontend-postgres/README.md) | Two-component recipe |
| [global-app-layer/realistic-app](./global-app-layer/realistic-app/README.md) | Three-component app |
| [global-app-layer/gpu-eks-h100-training](./global-app-layer/gpu-eks-h100-training/README.md) | NVIDIA AICR-shaped stack |

### App-Style GitOps Layouts

| Example | What it proves |
|---------|----------------|
| [apptique-argo-app-of-apps](./apptique-argo-app-of-apps/README.md) | Argo app-of-apps layout |
| [apptique-argo-applicationset](./apptique-argo-applicationset/README.md) | Argo ApplicationSet layout |
| [apptique-flux-monorepo](./apptique-flux-monorepo/README.md) | Flux monorepo layout |
| [flux-boutique](./flux-boutique/README.md) | Flux multi-service fan-out |

### Offline and Simulation

| Example | What it proves |
|---------|----------------|
| [demo-data-adt](./demo-data-adt/README.md) | Scan-first ADT example |
| [lifecycle-hazards](./lifecycle-hazards/README.md) | Migration risk detection |
| [fleet-import](./fleet-import/README.md) | Multi-cluster aggregation |
| [import-from-bundle](./import-from-bundle/README.md) | Offline bundle import |

## Delivery Matrix

| Mode | Status | Use case |
|------|--------|----------|
| **Direct Kubernetes** | Fully working | Simplest real proof |
| **Flux OCI** | Current standard | Controller-oriented delivery |
| **Argo OCI** | Implemented | Use with controller + live evidence |
| **ArgoCDRenderer** | Working | Renderer path only, not OCI delivery |

## Reality Guide

- **100% real e2e**: `global-app-layer/*` examples with non-Noop targets
- **Real mutation, no live delivery**: `platform-write-api`
- **Real import, no ConfigHub apply**: `gitops-import-*`, `combined-git-live`
- **Simulation only**: `demo-data-adt`, `lifecycle-hazards`

## Rules

- Keep changes additive and easy to diff
- Include verification commands
- Do not break existing root examples
- For major examples, follow [`ai-example-playbook.md`](./ai-example-playbook.md)
- All runnable examples must pass [`ai-guide-standard.md`](./ai-guide-standard.md)
