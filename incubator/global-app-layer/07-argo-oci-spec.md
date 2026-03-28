# Argo OCI Specification

**Status:** Target-state design. Not yet implemented.
**ArgoCD Support:** Confirmed. ArgoCD v3.1+ (August 2025) has native OCI support.

This document specifies the Argo OCI delivery path for ConfigHub. It defines what needs to be built for Argo to have the same OCI bundle contract as Flux OCI.

## Why This Spec Exists

Flux OCI is the current standard for controller-oriented bundle delivery. Argo should have an equivalent path. This spec defines that path without over-implementing.

The goal is to make Argo OCI a first-class delivery mode where:

- ConfigHub publishes an OCI artifact
- Argo `Application` consumes that artifact
- Argo reconciles workloads from the published bundle
- The same bundle contract applies as for Flux OCI

## What This Is Not

**ArgoCDRenderer** is not Argo OCI. It is a renderer path that:

- Expects Argo `Application` CRD payloads (not raw manifests)
- Sends Applications to ArgoCD for hydration
- Does not publish OCI bundles
- Does not reconcile workloads

Do not conflate ArgoCDRenderer with this spec. They are different paths with different purposes.

## Target/Provider Shape

### Provider Name

```
ArgoOCI
```

or

```
ArgoOCIWriter
```

### Target Slug Pattern

```
worker-argooci-kubernetes-yaml-cluster
```

### Provider Type

```yaml
providerType: ArgoOCI
deliveryMode: argo-oci
```

## Unit Payload Shape

The unit payload should be **raw Kubernetes manifests**, the same as for Direct Kubernetes and Flux OCI.

```yaml
# Unit payload (example)
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: production
spec:
  replicas: 3
  # ...
```

This is different from ArgoCDRenderer, which expects:

```yaml
# ArgoCDRenderer payload (NOT for Argo OCI)
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  source:
    repoURL: https://github.com/...
  # ...
```

## OCI Publication Contract

When `cub unit apply` is called with an ArgoOCI target, the worker should:

1. **Render** the unit's current data
2. **Package** the rendered manifests as an OCI artifact
3. **Push** the artifact to the configured OCI registry
4. **Record** the publication facts:

```yaml
bundlePublication:
  uri: oci://registry.example.com/bundles/my-app
  digest: sha256:abc123...
  publishedAt: 2026-03-28T12:00:00Z
  target: my-space/worker-argooci-kubernetes-yaml-cluster
  deploymentVariant: my-app-production
  recipeManifest: recipe-us-staging/recipe-manifest
  unitRevision: 42
```

## Argo Application Source Contract

The worker should create or update an Argo `Application` that points at the published OCI artifact:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
  labels:
    confighub.com/managed-by: worker
    confighub.com/bundle-digest: sha256:abc123...
spec:
  project: default
  source:
    repoURL: oci://registry.example.com/bundles/my-app
    targetRevision: sha256:abc123...  # exact digest
    path: .
  destination:
    server: https://kubernetes.default.svc
    namespace: production
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

Key requirements:

- Source is the OCI artifact, not a Git repo
- `targetRevision` is the exact digest from publication
- Labels connect the Application back to ConfigHub
- SyncPolicy enables Argo to manage the workload lifecycle

## Verification Contract

A real Argo OCI proof must show all of these:

### 1. Publication Evidence

```bash
# OCI artifact exists with correct digest
crane manifest oci://registry.example.com/bundles/my-app@sha256:abc123...
```

### 2. Argo Application Evidence

```bash
# Application exists and points at the OCI artifact
kubectl get application my-app -n argocd -o yaml
# Should show:
#   source.repoURL: oci://...
#   source.targetRevision: sha256:abc123...
```

### 3. Sync Evidence

```bash
# Application is synced and healthy
kubectl get application my-app -n argocd -o jsonpath='{.status.sync.status}'
# Should return: Synced

kubectl get application my-app -n argocd -o jsonpath='{.status.health.status}'
# Should return: Healthy
```

### 4. Cluster Evidence

```bash
# Workloads match the bundle content
kubectl get deployment my-app -n production -o yaml
# Should match the manifests in the OCI artifact
```

### 5. Digest Chain Evidence

The verification should prove the chain:

```
ConfigHub unit revision → OCI artifact digest → Argo targetRevision → deployed workload
```

## Implementation Checklist

| Component | Status | Notes |
|-----------|--------|-------|
| ArgoOCI provider type | Not implemented | New worker provider type |
| OCI publication logic | Partially exists | Reuse from FluxOCI |
| Argo Application creation | Not implemented | New worker logic |
| Application source contract | Spec only | Needs Argo OCI support |
| Verification helpers | Not implemented | Need new verification scripts |
| Example deployment variant | Not implemented | Add to `gpu-eks-h100-training` |

## Comparison With Flux OCI

| Aspect | Flux OCI | Argo OCI |
|--------|----------|----------|
| Status | Current standard | Target-state |
| Unit payload | Raw K8s manifests | Raw K8s manifests |
| OCI publication | Worker publishes | Worker publishes |
| Controller resource | OCIRepository + Kustomization | Application with OCI source |
| Reconciliation | Flux manages | Argo manages |
| Sync evidence | Kustomization status | Application sync/health |

## What Counts As Real Proof

A real Argo OCI proof must demonstrate:

1. **Publication**: OCI artifact pushed with recorded digest
2. **Application**: Argo Application created with OCI source pointing at exact digest
3. **Sync**: Application reports Synced and Healthy
4. **Workload**: Cluster resources match bundle content
5. **Lineage**: Clear chain from ConfigHub unit to deployed workload

Until all five are demonstrated, Argo OCI is not proven.

## Open Questions

### 1. Argo OCI Source Support — ANSWERED

**Yes.** ArgoCD v3.1+ (August 2025) has native OCI support.

Application spec format:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
  namespace: argocd
spec:
  source:
    repoURL: oci://registry.example.com/bundles/my-app
    targetRevision: sha256:abc123...  # or tag
    path: .
  destination:
    server: "https://kubernetes.default.svc"
    namespace: production
```

Requirement: ArgoCD v3.1 or later.

Sources:
- [Argo CD OCI User Guide](https://argo-cd.readthedocs.io/en/latest/user-guide/oci/)
- [Argo CD v3.1 OCI Support Announcement](https://www.infoq.com/news/2025/08/argocd-oci-support-new-ui/)

### 2. Registry Configuration

How should the worker discover or configure the OCI registry for Argo?

- Per-target configuration?
- Space-level configuration?
- Worker-level default?

### 3. Application Namespace

Should the Application live in the ArgoCD namespace or alongside workloads?

- Recommendation: ArgoCD namespace (`argocd`) for consistency

### 4. SyncPolicy

Should ConfigHub control sync policy, or leave it to operator configuration?

- Recommendation: Default to automated sync, configurable per target

## Next Steps

1. Confirm Argo OCI source support and version requirements
2. Implement ArgoOCI provider type in worker
3. Add ArgoOCI deployment variant to `gpu-eks-h100-training`
4. Add verification scripts for Argo sync/health evidence
5. Document the end-to-end proof flow

## Related Documents

- [how-it-works.md](./how-it-works.md) - Delivery mechanics
- [contracts.md](./contracts.md) - Read-only inspection contracts
- [05-bundle-publication-walkthrough.md](./05-bundle-publication-walkthrough.md) - Bundle story stages
- [gpu-eks-h100-training/README.md](./gpu-eks-h100-training/README.md) - Reference implementation (for Flux OCI)
