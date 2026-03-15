# GitOps Import (Argo CD)

This directory contains scripts and data to set up a test environment for testing GitOps import on ConfigHub

It sets up a local kind cluster with Argo CD and a representative gitops config repo ([jesperfj/gitops-argocd](https://github.com/jesperfj/gitops-argocd)) containing:

- **2 Argo CD Projects**: `app-cubbychat` (application team) and `platform` (infrastructure)
- **2 ApplicationSets**: one per project, using git directory generators
- **7 Applications**: `cubbychat` (backend/frontend/postgres), `alloy`, `cert-manager`, `external-dns`, `external-secrets`, `grafana`, `traefik`

## Setup

```bash
bin/create-cluster       # Create kind cluster (kubeconfig saved to var/)
bin/install-argocd       # Install Argo CD with direct localhost access
bin/setup-apps           # Create projects and apply ApplicationSets
CUB_SPACE=<space> bin/install-worker   # Create and deploy ConfigHub worker
```

## Access

- **Argo CD UI**: http://localhost:9080
- **Username**: admin
- **Password**: `cat var/argocd-admin-password.txt`
- **Kubeconfig**: `export KUBECONFIG=$PWD/var/gitops-import.kubeconfig`

## GitOps Discover / Import

After setup, the worker registers two targets:

- `worker-kubernetes-yaml-cluster` — Kubernetes discovery target
- `worker-argocdrenderer-kubernetes-yaml-cluster` — Argo CD renderer target

```bash
# Discover ArgoCD Applications
cub gitops discover --space <space> worker-kubernetes-yaml-cluster

# Import into ConfigHub
cub gitops import --space <space> worker-kubernetes-yaml-cluster worker-argocdrenderer-kubernetes-yaml-cluster
```

## Teardown

```bash
bin/teardown
```
