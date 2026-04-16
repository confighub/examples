# GitOps Import (Argo CD)

This directory contains scripts and data to set up a test environment for GitOps import on ConfigHub with Argo CD. For background and a walkthrough, see:

- [GitOps Import with ArgoCD](https://docs.confighub.com/get-started/examples/gitops-import/) — the tutorial that accompanies this example
- [Render manifests from DRY formats](https://docs.confighub.com/guide/rendered-manifests/) — what `cub gitops discover` / `import` does under the covers, end to end

It sets up a local kind cluster with Argo CD and a representative gitops config repo ([jesperfj/gitops-argocd](https://github.com/jesperfj/gitops-argocd)) containing:

- **2 Argo CD Projects**: `app-cubbychat` (application team) and `platform` (infrastructure)
- **2 ApplicationSets**: one per project, using git directory generators
- **3 Applications** generated from the upstream repo: `cubbychat` (app), `alloy` and `grafana` (platform)

## Setup

```bash
bin/create-cluster       # Create kind cluster (kubeconfig saved to var/)
bin/install-argocd       # Install Argo CD with direct localhost access
bin/setup-apps           # Create projects and apply ApplicationSets
CUB_SPACE=<space> bin/install-worker   # Create and deploy ConfigHub worker
```

The worker is installed with three bridge capabilities: `kubernetes`, `argocdrenderer`, and `argocdoci`. Those capabilities register on a single target, `worker-kubernetes-yaml-cluster`, as additional `ConfigTypes`. A unit's `ProviderType` selects which bridge handles it — so dry units (`ArgoCDRenderer`) and wet units (`ArgoCDOCI`) share the same target.

## Access

- **Argo CD UI**: http://localhost:9080
- **Username**: admin
- **Password**: `cat var/argocd-admin-password.txt`
- **Kubeconfig**: `export KUBECONFIG=$PWD/var/gitops-import.kubeconfig`

## GitOps Discover / Import

After setup, the worker registers the target `worker-kubernetes-yaml-cluster`. A single target is sufficient for discovery, rendering, and deployment:

```bash
# Discover ArgoCD Applications in the cluster
cub gitops discover --space <space> worker-kubernetes-yaml-cluster

# Import: creates dry units (renderer inputs) and wet units (rendered output),
# linked so applying a dry unit populates its wet unit's Data.
cub gitops import --space <space> worker-kubernetes-yaml-cluster
```

After import, the discovered Argo CD `Application` resources have auto-sync disabled so ConfigHub owns the rendered output. Do not delete them or sync them manually — applying the wet units deploys a new Argo CD `Application` that fetches rendered manifests from ConfigHub's OCI registry and takes over the workload.

See [Render manifests from DRY formats](https://docs.confighub.com/guide/rendered-manifests/) for the full flow, including how to re-render after a change in git and how CRDs are split out.

## Applying ConfigHub units to this cluster

You can also use this cluster to apply ConfigHub-managed units directly (e.g. from the layered recipe examples). `cub unit apply` does not create namespaces automatically, but you can create the namespace as a ConfigHub unit and link your workload unit to it so ordering is handled for you:

```bash
export KUBECONFIG=$PWD/var/gitops-import.kubeconfig

# Create and apply a namespace unit
kubectl create namespace <namespace> -o yaml --dry-run=client \
  | egrep -v "creationTimestamp|status" \
  | cub unit create --space <deploy-space> <namespace-unit> -
cub unit apply --space <deploy-space> <namespace-unit>

# Link the workload unit to the namespace unit, then apply
cub link create --space <deploy-space> - <deploy-unit> <namespace-unit>
cub unit apply --space <deploy-space> <deploy-unit>
```

## Teardown

```bash
bin/teardown
```
