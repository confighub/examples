#!/bin/bash

set -euo pipefail

CLUSTER_NAME=${1:-gitops-import-argo}
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

echo "==> Checking ArgoCD namespace"
kubectl get namespace argocd >/dev/null

echo "==> Checking ArgoCD applications"
kubectl get applications -n argocd

echo "==> Checking ConfigHub worker deployment if present"
if kubectl get namespace confighub >/dev/null 2>&1; then
    kubectl get deployment -n confighub
else
    echo "confighub namespace not present yet"
fi

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
else
    echo "cub-scout not installed; skipping optional live ownership checks"
fi

echo "All gitops-import-argo checks passed."
