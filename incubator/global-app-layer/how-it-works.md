# How This Works

This document explains the mechanics behind the four `global-app-layer` examples: how manifests move through ConfigHub, how name conflicts are avoided, what the worker does, and where AI fits.

## 1. Config Manifests live in the Database

Every piece of Kubernetes YAML lives in ConfigHub as a **unit** — a versioned, content-addressed blob. The lifecycle is:

```mermaid
flowchart LR
  File["Manifest file in Git or local disk"] --> Base["Base unit"]
  Base --> Variant1["Variant unit"]
  Variant1 --> Variant2["Variant unit"]
  Variant2 --> Deploy["Deployment unit"]
  Deploy --> Target["Target binding"]
  Target --> Apply["Worker apply or publish"]
```

The user-facing idea is a **layered variant chain**:

- one base unit
- one more specialized variant at each layer
- one final deployment unit

The underlying ConfigHub mechanism is a set of **clone links** between those units. That is how shared updates can flow downstream with provenance.

### `single-component`: one layered variant chain

```mermaid
flowchart LR
  File["backend.yaml"] --> Base["backend-base<br/>catalog-base"]
  Base --> Region["backend-us<br/>catalog-us"]
  Region --> Role["backend-us-staging<br/>catalog-us-staging"]
  Role --> Recipe["backend-recipe-us-staging<br/>recipe-us-staging"]
  Recipe --> Deploy["backend-cluster-a<br/>deploy-cluster-a"]
```

Layer-by-layer, that means:

- `backend-base`: original manifest stored as a unit
- `backend-us`: region specialization
- `backend-us-staging`: role specialization
- `backend-recipe-us-staging`: recipe-level shared intent
- `backend-cluster-a`: final deployment-specific variant

Each variant is a new revision that inherits its parent’s data. Mutations are applied via `cub function do`, which writes a new revision to the unit. The database tracks every revision with a content hash.

The **recipe manifest** unit is separate metadata in the recipe space. It is not another executable layer. It is the receipt that records the full provenance: which spaces, units, revisions, and hashes compose the recipe.

When you change the base, the change propagates down through clone links. When you mutate a leaf, only that leaf changes. The database holds the complete history.

## 2. How NVIDIA AICR Layers Map to ConfigHub

NVIDIA AICR talks about layers.
ConfigHub stores one unit at each specialization stage.

```mermaid
flowchart TB
  subgraph AICR["AICR recipe"]
    A0["Base component"]
    A1["Platform layer"]
    A2["Accelerator layer"]
    A3["OS layer"]
    A4["Intent layer"]
    A5["Deployment overlay"]
    A0 --> A1 --> A2 --> A3 --> A4 --> A5
  end

  subgraph CH["ConfigHub representation"]
    C0["Base unit"]
    C1["Platform variant"]
    C2["Accelerator variant"]
    C3["OS variant"]
    C4["Recipe variant"]
    C5["Deployment variant"]
    C0 --> C1 --> C2 --> C3 --> C4 --> C5
  end

  A0 -. "stored as" .-> C0
  A1 -. "stored as" .-> C1
  A2 -. "stored as" .-> C2
  A3 -. "stored as" .-> C3
  A4 -. "stored as" .-> C4
  A5 -. "stored as" .-> C5
```

For the GPU example that becomes:

```mermaid
flowchart LR
  G0["gpu-operator-base"] --> G1["gpu-operator-eks"] --> G2["gpu-operator-eks-h100"] --> G3["gpu-operator-eks-h100-ubuntu"] --> G4["gpu-operator-eks-h100-ubuntu-training"] --> G5["gpu-operator-cluster-a"]
  P0["nvidia-device-plugin-base"] --> P1["nvidia-device-plugin-eks"] --> P2["nvidia-device-plugin-eks-h100"] --> P3["nvidia-device-plugin-eks-h100-ubuntu"] --> P4["nvidia-device-plugin-eks-h100-ubuntu-training"] --> P5["nvidia-device-plugin-cluster-a"]
```

The important shift is:

- AICR describes a stack as layers
- ConfigHub stores each layer outcome as a real unit
- clone links record how one specialized variant came from an earlier one

## 3. GitOps with Workers and Targets

Traditional GitOps sources YAML from Git. ConfigHub is different: config units are literal deployment manifests that are **deployed to targets** — named delivery endpoints.

All targets are created, registered, and managed using **workers**. Each **worker** is a long-lived agent running inside the Kubernetes cluster (in the `confighub` namespace). It maintains a persistent connection to the ConfigHub management plane and registers one or more **targets**, scoped to the worker's space.

When you `cub unit set-target` + `cub unit apply`:

1. ConfigHub renders the unit's current data (the accumulated base + all clone mutations)
2. Sends it to the worker via the target reference
3. The worker applies the rendered YAML to the cluster

Important:

- a visible target is not enough for a believable demo
- a good demo should show that the worker is actually ready before mutation
- in this package, `./preflight-live.sh <space/target>` is the safest way to show that readiness

The worker supports two target types in this example:

| Target | Slug | What it does |
|---|---|---|
| **Kubernetes (direct)** | `worker-kubernetes-yaml-cluster` | Worker applies YAML directly via `kubectl apply` |
| **ArgoCDRenderer** | `worker-argocdrenderer-kubernetes-yaml-cluster` | Worker manages Argo CD `Application` CRDs and asks ArgoCD to render them |

**Payload compatibility matters:**

| Example | Kubernetes Target | ArgoCDRenderer Target |
|---------|-------------------|----------------------|
| `realistic-app` | ✅ | ❌ (raw manifests, not Application CRDs) |
| `single-component` | ✅ | ❌ |
| `frontend-postgres` | ✅ | ❌ |
| `gpu-eks-h100-training` | ✅ | ❌ |
| Brownfield-imported Applications | ✅ | ✅ |

In these examples the direct target is the most fully proven delivery path today.
The layered variant chain is unchanged conceptually when you change executors, but the unit payload still has to match the target type. The raw-manifest examples in this package are not yet a one-line swap to `ArgoCDRenderer`, and a good delegated-delivery proof still needs explicit Argo-side evidence, not only target binding.

### How ArgoCD Integration Works

When you use the `ArgoCDRenderer` target, the flow is:

```
ConfigHub (materialized Application)
    → worker (in-cluster agent)
        → ArgoCD API (render/hydrate)
            → rendered manifests returned to ConfigHub
```

The current `ArgoCDRenderer` implementation works with Argo CD `Application` resources, not arbitrary raw Kubernetes manifests. It also disables autosync so ArgoCD does not become the workload reconciler for this path. So for an Application-shaped unit, ConfigHub can hand that object to the worker, let ArgoCD render it, and then read back Argo-side evidence about the rendered manifests. But the raw-manifest chains in `realistic-app` and `gpu-eks-h100-training` do not yet fit that target directly.

That is still different from the typical ArgoCD model where Argo watches a Git repo and reconciles workloads. Here, ArgoCD is being used as a render/hydration engine. A separate sync-capable target path is still needed for a true Argo-managed workload proof.

That distinction matters for demos:

- if the proof is about direct worker delivery, show worker readiness plus cluster results
- if the proof is about Argo rendering, show worker readiness, Argo `Application` objects, and rendered-manifest evidence
- do not call `ArgoCDRenderer` a real Argo-sync proof for workloads
- do not treat a hybrid helper that runs `kubectl apply` and then creates an Argo Application as the same thing as a real Argo-backed target proof

### Brownfield: The Reverse Direction

The brownfield flow goes the other way — `cub gitops discover` finds existing ArgoCD Applications on the cluster and `cub gitops import` pulls their rendered manifests into ConfigHub as units. That's Git→Argo→ConfigHub (one-time import). After that, the ongoing renderer flow is ConfigHub→worker→Argo render API→rendered manifests.

### Label Mapping (Open Design Question)

When ArgoCD Applications carry labels like `team=payments` or `env=prod`, there is currently no deterministic convention for how those map to ConfigHub spaces and unit labels. This is a [known gap with a proposed design](../planning/2026-03-17-label-mapping-convention.md). The labels are preserved in imported YAML but don't yet influence ConfigHub organizational placement.

## 4. End-to-End Testing

The [e2e/](./e2e/) directory now contains the full lifecycle tests for this package.

It covers:

- **Brownfield**: import an existing cluster app into ConfigHub, mutate it, apply
- **Greenfield**: create layered chains from scratch, deploy all four recipes
- **Bridge**: import first, then layer greenfield config on top
- **Direct delivery**: apply a single materialized example through the worker
- **Argo-oriented delivery**: compare the hybrid helper path with the renderer-oriented `ArgoCDRenderer` target, while keeping real Argo-managed sync as a separate future proof

## 5. Role of AI

ConfigHub does **not use AI internally** but we recommend using it to "drive" the management plane and config data APIs.

ConfigHub mutations are explicit, deterministic function calls:

| Function | What it does |
|---|---|
| `set-env` | Set an environment variable on a container |
| `set-replicas` | Set replica count on a Deployment/StatefulSet |
| `set-namespace` | Set the metadata.namespace on all resources |
| `set-string-path` | Set any string value at an arbitrary YAML path |

Same input always produces same output. No model inference, no probabilistic output, no training data dependency.

Where AI fits in the ConfigHub vision is **operating assistance** — helping humans design layered variant chains, choose which mutations to apply at which layer, generate recipe manifests, and reason about config drift. Eg Helm is a deterministic generator that could be AI-assisted (e.g., "given this Helm chart, propose a layered chain").

In this demo our example scripts were written with AI assistance (designing the layering structure, debugging deployment failures, creating stub dependencies, iterating on the chain design). But the artifacts produced are plain bash scripts calling deterministic CLI commands. There is no AI in the loop at runtime.

## 6. Avoiding Name Conflicts

Each recipe run gets a random prefix (e.g. `hug-hug`, `den-cub`, `roll-cub`, `berry-sun`) generated by `cub space new-prefix`. Every space name is `{prefix}-{suffix}`:

- `hug-hug-catalog-base`, `hug-hug-deploy-cluster-a` (single-component)
- `den-cub-catalog-base`, `den-cub-deploy-cluster-a` (frontend-postgres)

This means four recipes create ~20 spaces with zero name collisions.

On the cluster side, `DEPLOY_NAMESPACE` is overridable (`recipe-sc`, `recipe-fp`, `recipe-ra`, `recipe-gpu`), so each recipe's pods land in a different namespace.

Unit names within a space are fixed per-component (`backend-base`, `frontend-cluster-a`) — they don't conflict because they live in prefix-scoped spaces.
