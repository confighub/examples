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

echo "==> Checking the healthy reference applications"
kubectl -n argocd get application cubbychat >/dev/null
kubectl -n argocd get application helm-guestbook >/dev/null
kubectl -n argocd get application kustomize-guestbook >/dev/null

echo "==> Checking ArgoCD applications"
kubectl get applications -n argocd

echo "==> Checking ConfigHub worker deployment if present"
if [ -f "$PROJECT_DIR/var/worker.pid" ]; then
    echo "local worker pid: $(cat "$PROJECT_DIR/var/worker.pid")"
    if [ -f "$PROJECT_DIR/var/worker.log" ]; then
        echo "local worker log: $PROJECT_DIR/var/worker.log"
    fi
    if [ -f "$PROJECT_DIR/var/argocd-port-forward.pid" ]; then
        echo "argocd port-forward pid: $(cat "$PROJECT_DIR/var/argocd-port-forward.pid")"
    fi
else
    echo "local worker pid file not present"
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

echo "Verification completed."
echo "This example expects a mixed ArgoCD state: a few healthy reference applications and several deliberately failing contrast applications."
echo "Treat the failing ArgoCD objects above as evidence, not as a script failure."
