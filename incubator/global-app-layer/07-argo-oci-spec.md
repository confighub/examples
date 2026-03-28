# Argo OCI Specification

**Status:** Implementation exists. Ready for example variants.
**Provider Type:** `ArgoCDOCI`
**ArgoCD Support:** Confirmed. ArgoCD v3.1+ (August 2025) has native OCI support.

This document specifies the Argo OCI delivery path for ConfigHub. The worker implementation already exists at `confighub/public/bridge-impl/argocd/argocd_oci.go`.

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
ArgoCDOCI
```

### Target Slug Pattern

```
worker-argocdoci-kubernetes-yaml-cluster
```

### Provider Type

```yaml
providerType: ArgoCDOCI
```

### Implementation Location

```
confighub/public/bridge-impl/argocd/argocd_oci.go
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
| ArgoCDOCI provider type | **Exists** | `confighub/public/bridge-impl/argocd/argocd_oci.go` |
| OCI publication logic | Exists | Built into ArgoCDOCI provider |
| Argo Application creation | Exists | Auto-generates Application CRs |
| Application source contract | Exists | ArgoCD v3.1+ OCI source format |
| Repo credentials | Exists | Auto-generates repo-creds Secret |
| Verification helpers | Needed | Add to example scripts |
| Example deployment variant | **Ready to add** | No blockers |

## Existing Worker Implementation

The `ArgoCDOCI` provider already exists and implements:

```go
type ArgoCDOCIWorker struct {
    KubernetesBridgeWorker
    workerID     string  // For OCI auth
    workerSecret string  // For OCI auth
}
```

### Key Features

- **OCI URL handling**: Auto-constructs `oci://host/confighub/space-slug/unit-slug:reference`
- **Application generation**: Creates ArgoCD Application CRs with OCI source
- **Repo credentials**: Auto-generates `confighub-oci-creds-{host}` Secret
- **Sync monitoring**: Tracks sync status, health status, and operation state
- **Drift detection**: Detects content drift via diff-patch analysis
- **Helm support**: Auto-detects Helm units and generates Helm-style Applications

### Configuration Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `ArgoCDNamespace` | `argocd` | Where to create Application CRs |
| `DestinationServer` | `https://kubernetes.default.svc` | Target cluster |
| `DestinationNamespace` | `default` | Target namespace |
| `Project` | `default` | ArgoCD project |
| `SyncPolicy` | `manual` | `automated` or `manual` |
| `PruneEnabled` | false | ArgoCD prune option |
| `SelfHealEnabled` | false | ArgoCD self-heal option |

### Adding Example Variant

Now that the provider exists, adding the variant is straightforward:

```bash
# In lib.sh
DEPLOY_ARGO_SPACE_SUFFIX="deploy-cluster-a-argo"
DEPLOY_ARGO_UNIT="backend-cluster-a-argo"
DEPLOY_VARIANTS=(direct flux argo)

# In setup.sh
create_space_if_missing "$(argo_deploy_space)" "${argo_deploy_space_labels[@]}"
create_clone_unit "$(argo_deploy_space)" "${DEPLOY_ARGO_UNIT}" "$(recipe_space)" "${RECIPE_UNIT}"

# In set-target.sh
case "${provider_type}" in
  ArgoCDOCI) variant="argo" ;;
esac
```

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

1. ~~Confirm Argo OCI source support and version requirements~~ **DONE** — ArgoCD v3.1+ confirmed
2. ~~Implement ArgoCDOCI provider type in worker~~ **EXISTS** — `confighub/public/bridge-impl/argocd/argocd_oci.go`
3. ~~Add ArgoCDOCI deployment variant to `single-component`~~ **DONE**
4. ~~Add ArgoCDOCI deployment variant to `gpu-eks-h100-training`~~ **DONE**
5. Add verification scripts for Argo sync/health evidence
6. Document the end-to-end proof flow

**Remaining gaps:** shared verification helpers, package-level proof wording, and end-to-end evidence capture.

## Related Documents

- [how-it-works.md](./how-it-works.md) - Delivery mechanics
- [contracts.md](./contracts.md) - Read-only inspection contracts
- [05-bundle-publication-walkthrough.md](./05-bundle-publication-walkthrough.md) - Bundle story stages
- [gpu-eks-h100-training/README.md](./gpu-eks-h100-training/README.md) - Reference implementation (for Flux OCI)
