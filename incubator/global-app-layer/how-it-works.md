# How This Works

This document explains the mechanics behind the four `global-app-layer` examples: how manifests move through ConfigHub, how name conflicts are avoided, what the worker does, and where AI fits.

## 1. Config Manifests live in the Database

Every piece of Kubernetes YAML lives in ConfigHub as a **unit** — a versioned, content-addressed blob. The lifecycle is:

```
Base unit (create from file)
  → Clone from unit (clone inherits parent data)
    → Mutate (functions modify the clone)
      → Target (bind to a delivery endpoint)
        → Apply (worker pushes rendered YAML to cluster)
```

Concretely, for the `single-component` example, the backend goes through this **chain**:

```
backend.yaml  →  [backend-base]              (catalog-base space)
                       ↓ clone
                  [backend-us]                (catalog-us space)
                  + set-env REGION=US
                  + set ingress host
                       ↓ clone
                  [backend-us-staging]        (catalog-us-staging space)
                  + set-replicas 2
                  + set-env ROLE=staging
                       ↓ clone
                  [backend-recipe-us-staging] (recipe-us-staging space)
                  + set-env CHAT_TITLE=...
                       ↓ clone
                  [backend-cluster-a]         (deploy-cluster-a space)
                  + set-namespace
                  + set-env CLUSTER=...
```

Each clone is a new revision that inherits its parent's data. Mutations are applied via `cub function do`, which writes a new revision to the unit. The database tracks every revision with a content hash.

The **recipe manifest** unit is a separate metadata unit in the recipe space that records the full provenance: which spaces, units, revisions, and hashes compose the recipe.

When you change the base, the change propagates down through clone links. When you mutate a leaf, only that leaf changes. The database holds the complete history.

## 2. GitOps with Workers and Targets

Whereas YAMLs are **sourced** from Git, config units are literal deployment manifests that are **deployed to targets** - named delivery endpoints.  All targets are created, registered, and managed using **workers**.  Eacg **worker** is a long-lived agent running inside the Kubernetes cluster (in the `confighub` namespace). It maintains a persistent connection to the ConfigHub management plane to register one or more **targets**, which are scoped to the worker's space.

When you `cub unit set-target` + `cub unit apply`:

1. ConfigHub renders the unit's current data (the accumulated base + all clone mutations)
2. Sends it to the worker via the target reference
3. The worker applies the rendered YAML to the cluster

The worker supports two target types in this example:

| Target | Slug | What it does |
|---|---|---|
| **Kubernetes (direct)** | `worker-kubernetes-yaml-cluster` | Worker applies YAML directly via `kubectl apply` |
| **ArgoCDRenderer** | `worker-argocdrenderer-kubernetes-yaml-cluster` | Worker pushes YAML through ArgoCD |

In these examples we use the direct target. You could swap the direct target for ArgoCD and the recipe chain is unchanged — only the delivery mode differs.

### How ArgoCD Integration Works

When you use the `ArgoCDRenderer` target, the flow is:

```
ConfigHub (materialized config)
    → worker (in-cluster agent)
        → ArgoCD (as a rendering/delivery engine)
            → cluster
```

The worker hands ArgoCD the rendered YAML that ConfigHub materialized through the clone chain. ArgoCD applies it and then does what ArgoCD does — drift detection, self-heal, health checks, sync status. But the **source of truth is ConfigHub**, not a Git repo.

This is the opposite of the typical ArgoCD model where Argo watches a Git repo. Here, ArgoCD is demoted from "source of truth" to "delivery and reconciliation engine."

## 3. End-to-End Testing

### Brownfield: The Reverse Direction

The brownfield flow is the reverse — `cub gitops discover` finds existing ArgoCD Applications on the cluster and `cub gitops import` pulls their rendered manifests into ConfigHub as units. That's Git→Argo→ConfigHub (one-time import). After that, the ongoing flow is ConfigHub→worker→Argo→cluster.

### Label Mapping (Open Design Question)

When ArgoCD Applications carry labels like `team=payments` or `env=prod`, there is currently no deterministic convention for how those map to ConfigHub spaces and unit labels. This is a [known gap with a proposed design](../planning/2026-03-17-label-mapping-convention.md) — the labels are preserved in imported YAML but don't yet influence ConfigHub organizational placement. See [PR #20](https://github.com/confighub/examples/pull/20) for the proposed label-map spec.

The [e2e/](./e2e/) directory has delivery scripts for applying a single example to a cluster (direct or via ArgoCD). The [incubator/e2e/](../e2e/) directory has full lifecycle tests that cover three flows:

- **Brownfield**: import an existing cluster app into ConfigHub, mutate it, apply
- **Greenfield**: create layered chains from scratch, deploy all four recipes
- **Bridge**: import first, then layer greenfield config on top

## 4. Role of AI

**AI has zero role in the runtime data path.** ConfigHub does not use AI to generate, transform, or decide anything about your Kubernetes manifests.

The mutations are explicit, deterministic function calls:

| Function | What it does |
|---|---|
| `set-env` | Set an environment variable on a container |
| `set-replicas` | Set replica count on a Deployment/StatefulSet |
| `set-namespace` | Set the metadata.namespace on all resources |
| `set-string-path` | Set any string value at an arbitrary YAML path |

Same input always produces same output. No model inference, no probabilistic output, no training data dependency.

Where AI fits in the ConfigHub vision is **authoring assistance** — helping humans design clone chains, choose which mutations to apply at which layer, generate recipe manifests, and reason about config drift. The `cub-gen` tool is a deterministic generator that could be AI-assisted (e.g., "given this Helm chart, propose a layered chain") but the generator itself is parse-don't-guess, no ML.

These example scripts themselves were written with AI assistance (designing the layering structure, debugging deployment failures, creating stub dependencies, iterating on the chain design). But the artifacts produced are plain bash scripts calling deterministic CLI commands. There is no AI in the loop at runtime.


## 5. Avoiding Name Conflicts

Each recipe run gets a random prefix (e.g. `hug-hug`, `den-cub`, `roll-cub`, `berry-sun`) generated by `cub space new-prefix`. Every space name is `{prefix}-{suffix}`:

- `hug-hug-catalog-base`, `hug-hug-deploy-cluster-a` (single-component)
- `den-cub-catalog-base`, `den-cub-deploy-cluster-a` (frontend-postgres)

This means four recipes create ~20 spaces with zero name collisions.

On the cluster side, `DEPLOY_NAMESPACE` is overridable (`recipe-sc`, `recipe-fp`, `recipe-ra`, `recipe-gpu`), so each recipe's pods land in a different namespace.

Unit names within a space are fixed per-component (`backend-base`, `frontend-cluster-a`) — they don't conflict because they live in prefix-scoped spaces.


See [incubator/e2e/README.md](../e2e/README.md) for details.
