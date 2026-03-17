# Global App Layer — Layered Recipes for ConfigHub

Four working examples to show how ConfigHub can model "layered recipes".  These reproducible configuration recipes for combining multiple components correctly, using [NVIDIA AICR](https://developer.nvidia.com/blog/validate-kubernetes-for-gpu-infrastructure-with-layered-reproducible-recipes/) which is open source.  Here we are showing that ConfigHub gives you an **easy way to deploy NVIDIA patterns correctly** and then **to manage them** with updates, patches, integrations, customisations and fleets.

## What Is NVIDIA AICR and how does ConfigHub manage it?

Quoting from AICR "Every AI cluster running on Kubernetes requires a full software stack that works together, from low-level driver and kernel settings to high-level operator and workload configurations. You get one cluster working, and spend days getting the next one to match. Upgrade a component, and something else breaks. Move to a new cloud and start over. AI Cluster Runtime is a new open-source project designed to remove cluster configuration from the critical path. It publishes optimized, validated, and reproducible Kubernetes configurations as recipes you can deploy onto your clusters."

NVIDIA ships GPU software as "recipes" — curated, tested combinations of drivers, operators, and plugins for specific hardware/OS/cloud combinations. Their model is: start with a base component, layer on platform choices (EKS vs GKE), hardware choices (H100 vs A100), OS choices (Ubuntu vs RHEL), and workload intent (training vs inference). The result is a reproducible, auditable configuration.  The software is known as [NVIDIA AICR](https://developer.nvidia.com/blog/validate-kubernetes-for-gpu-infrastructure-with-layered-reproducible-recipes/).

ConfigHub also enables reproducible, auditable configurations, as well as additonal management, operational and compliance capabilities. Where NVIDIA creates a stack out of a sequence of 'layers', in ConfigHub we organise configurations into config objects ("units") which can be linked into a chain of dependent 'config clones' with added components (or 'variants').  This achieves the required 'layering'.  These can then be deployed, organised and managed by ConfigHub.  Below, we set out which examples are showcased.  After that read about how to do more with them.

## The Examples

These four examples prove it works, in increasing complexity:

### single-component

The simplest case: one app (`backend`), five layers, one stub dependency (`postgres`). Proves the clone chain model works end-to-end. The postgres stub is a ConfigHub unit — not a manual `kubectl apply` — so the entire deployment goes through ConfigHub.

### frontend-postgres

Two components (`frontend` + `postgres`) with a stub dependency (`backend`). The frontend's nginx config expects a `backend` upstream, so a minimal backend stub fills that gap. Proves that multi-component recipes work and that stub dependencies stay inside ConfigHub.

### realistic-app

Three components (`backend` + `frontend` + `postgres`) — no stubs needed because all dependencies are real components in the recipe. Proves a recognizable multi-tier app works end-to-end through the layered model.

### gpu-eks-h100-training

This is NVIDIA's actual layering model: `base → platform(EKS) → accelerator(H100) → OS(Ubuntu) → recipe(training) → deployment`. Two components (`gpu-operator` + `nvidia-device-plugin`) with six layers. Uses stub container images (`nginx:1.27-alpine`, `busybox:1.37`) so it runs on any cluster including local `kind`, but the structure is real — swap the images for NVIDIA's actual operator images and point at a GPU node pool and it works.  Proves ConfigHub can express the same structure as NVIDIA's AICR pattern with real units, real clone links, and real provenance tracking.  

If you can model NVIDIA's most complex recipe pattern in ConfigHub, you can model many layered patterns :-)

### Summary

Every example creates real ConfigHub spaces, units, and clone chains. Every example can deploy real pods to a Kubernetes cluster via `cub unit apply`. All four can run simultaneously in different namespaces with zero conflicts.

| Example | Components | Layers | What it proves |
|---|---|---|---|
| [single-component](./single-component/) | backend + postgres stub | 5 (base → region → role → recipe → deploy) | The chain model works end-to-end |
| [frontend-postgres](./frontend-postgres/) | frontend + postgres + backend stub | 5 | Dependencies can be stubs inside ConfigHub |
| [realistic-app](./realistic-app/) | backend + frontend + postgres | 5 | A real multi-tier app works without stubs |
| [gpu-eks-h100-training](./gpu-eks-h100-training/) | gpu-operator + nvidia-device-plugin | 6 (base → platform → accelerator → OS → recipe → deploy) | NVIDIA's actual layering model in ConfigHub |

## How It Works

ConfigHub is a database for organising software config manifests plus a management platform for executing operations.  Each config is organised into apps, which connect to Sources (eg GitHub) and may be deployed to Targets (eg Argo or Flux in Kubernetes).  Thus ConfigHub may be 'inserted' into an existing GitOps flow.  The management layer can then act as a point of operational control for all changes to the connected software.  

See [how-it-works.md](./how-it-works.md) for the full explanation of:

- Manifest lifecycle in the ConfigHub database
- How name conflicts are avoided across concurrent recipes
- The role of workers, targets, and GitOps delivery
- Where AI fits (and doesn't fit) in the runtime path
- How ArgoCD integration works when you swap the delivery target

## Quick Start

Each example follows the same script interface:

```bash
# Pick an example
cd single-component    # or frontend-postgres, realistic-app, gpu-eks-h100-training

# Create the full clone chain in ConfigHub
./setup.sh

# Verify all spaces, units, and links are correct
./verify.sh

# Optional: bind to a worker target and deploy to a cluster
./set-target.sh <target-ref>
cub unit apply --space <deploy-space> --unit <deploy-unit>

# Optional: change the base and watch it propagate
./upgrade-chain.sh

# Tear down all spaces and units
./cleanup.sh
```

To deploy to a specific namespace (default is `cluster-a`):

```bash
DEPLOY_NAMESPACE=recipe-sc ./setup.sh
```

To run all four simultaneously:

```bash
DEPLOY_NAMESPACE=recipe-sc   ./single-component/setup.sh
DEPLOY_NAMESPACE=recipe-fp   ./frontend-postgres/setup.sh
DEPLOY_NAMESPACE=recipe-ra   ./realistic-app/setup.sh
DEPLOY_NAMESPACE=recipe-gpu  ./gpu-eks-h100-training/setup.sh
```

## Background Reading

These documents explain the design thinking behind the examples:

1. [01-nvidia-aicr-fit.md](./01-nvidia-aicr-fit.md) — Why the NVIDIA AICR pattern matters for ConfigHub
2. [02-recipes-and-layers-spec.md](./02-recipes-and-layers-spec.md) — The recipes-and-layers spec for ConfigHub
3. [04-review-and-next-steps.md](./04-review-and-next-steps.md) — Review of what works and what's still missing

## Prerequisites

- `cub` CLI, authenticated (`cub auth login`)
- `jq`
- For cluster deployment: a running Kubernetes cluster with a ConfigHub worker (see [gitops-import](../../gitops-import/) for setup)

## End-to-End Testing

- [e2e/](./e2e/) — delivery scripts for applying a single example to a cluster (direct or ArgoCD)
- [incubator/e2e/](../e2e/) — full lifecycle tests: brownfield import, greenfield deploy, and bridge (import then layer). See [incubator/e2e/README.md](../e2e/README.md).
