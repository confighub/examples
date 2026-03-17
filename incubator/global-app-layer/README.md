# Global App Layer — Layered Recipes for ConfigHub

Four working examples that prove ConfigHub can model layered, reproducible configuration recipes — from a single-component app all the way to NVIDIA's GPU infrastructure pattern.

## What This Is

NVIDIA ships GPU software as "recipes" — curated, tested combinations of drivers, operators, and plugins for specific hardware/OS/cloud combinations. Their model is: start with a base component, layer on platform choices (EKS vs GKE), hardware choices (H100 vs A100), OS choices (Ubuntu vs RHEL), and workload intent (training vs inference). The result is a reproducible, auditable configuration.

ConfigHub does the same thing with **spaces and clone chains**. These four examples prove it works, in increasing complexity:
This incubator package is the current home for some ConfigHub recipe and layer experiments, starting with `global-app` and extending into domain-shaped examples.  The layers are motivated by eg. the NVIDIA AICR OSS framework: https://developer.nvidia.com/blog/validate-kubernetes-for-gpu-infrastructure-with-layered-reproducible-recipes/

The project area has:

- analysis of the NVIDIA AICR layering mapped to ConfigHub
- a generalised working recipes-and-layers spec for ConfigHub
- four runnable worked examples that teach the model in stages

Quoting from AICR "Every AI cluster running on Kubernetes requires a full software stack that works together, from low-level driver and kernel settings to high-level operator and workload configurations. You get one cluster working, and spend days getting the next one to match. Upgrade a component, and something else breaks. Move to a new cloud and start over. AI Cluster Runtime is a new open-source project designed to remove cluster configuration from the critical path. It publishes optimized, validated, and reproducible Kubernetes configurations as recipes you can deploy onto your clusters."

## Start Here

| Example | Components | Layers | What it proves |
|---|---|---|---|
| [single-component](./single-component/) | backend + postgres stub | 5 (base → region → role → recipe → deploy) | The chain model works end-to-end |
| [frontend-postgres](./frontend-postgres/) | frontend + postgres + backend stub | 5 | Dependencies can be stubs inside ConfigHub |
| [realistic-app](./realistic-app/) | backend + frontend + postgres | 5 | A real multi-tier app works without stubs |
| [gpu-eks-h100-training](./gpu-eks-h100-training/) | gpu-operator + nvidia-device-plugin | 6 (base → platform → accelerator → OS → recipe → deploy) | NVIDIA's actual layering model in ConfigHub |

Every example creates real ConfigHub spaces, units, and clone chains. Every example can deploy real pods to a Kubernetes cluster via `cub unit apply`. All four can run simultaneously in different namespaces with zero conflicts.

**The point:** if you can model NVIDIA's most complex recipe pattern in ConfigHub, you can model anything.

## How It Works

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
