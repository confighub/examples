#!/bin/bash
set -euo pipefail

CLUSTER_NAME=${1:-gitops-import-flux}
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$SCRIPT_DIR"

export KUBECONFIG="$PROJECT_DIR/var/$CLUSTER_NAME.kubeconfig"

if [ ! -f "$KUBECONFIG" ]; then
    echo "Error: Kubeconfig not found at $KUBECONFIG"
    echo "Run bin/create-cluster first"
    exit 1
fi

echo "==> Checking cluster connectivity"
kubectl get nodes >/dev/null

echo "==> Checking Flux namespaces and controllers"
kubectl get namespace flux-system >/dev/null
kubectl get deployments -n flux-system

echo "==> Checking the healthy reference path"
kubectl -n flux-system get gitrepository podinfo >/dev/null
kubectl -n flux-system get kustomization podinfo >/dev/null
kubectl -n podinfo get deployment podinfo

echo "==> Checking Flux source and deployer objects"
kubectl get gitrepositories,kustomizations,helmreleases -A || true

echo "==> Checking ConfigHub targets if auth and space are available"
if [ -n "${CUB_SPACE:-}" ] && command -v cub >/dev/null 2>&1; then
    cub target list --space "$CUB_SPACE" --json | jq '.'
else
    echo "Skipping cub target check; set CUB_SPACE and ensure cub is installed and authenticated"
fi

echo "==> Optional cub-scout verification"
if command -v cub-scout >/dev/null 2>&1; then
    cub-scout gitops status || true
    cub-scout map list || true
    cub-scout tree ownership || true
else
    echo "cub-scout not installed; skipping optional live ownership checks"
fi

echo "Verification completed."
echo "This example expects one healthy Flux path (podinfo) and may also show failing contrast paths."
echo "Treat the failing Flux objects above as evidence, not as a script failure."
