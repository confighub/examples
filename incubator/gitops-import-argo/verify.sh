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
kubectl -n argocd get application helm-guestbook >/dev/null
kubectl -n argocd get application kustomize-guestbook >/dev/null

wait_for_application() {
    local app_name=$1
    local attempts=${2:-36}
    local delay=${3:-5}
    local status
    local health
    local attempt

    for attempt in $(seq 1 "$attempts"); do
        status=$(kubectl -n argocd get application "$app_name" -o jsonpath='{.status.sync.status}' 2>/dev/null || true)
        health=$(kubectl -n argocd get application "$app_name" -o jsonpath='{.status.health.status}' 2>/dev/null || true)
        if [ "$status" = "Synced" ] && [ "$health" = "Healthy" ]; then
            echo "$app_name is Synced and Healthy"
            return 0
        fi
        sleep "$delay"
    done

    echo "Timed out waiting for $app_name to become Synced and Healthy" >&2
    kubectl -n argocd get application "$app_name" -o wide >&2 || true
    return 1
}

wait_for_application helm-guestbook
wait_for_application kustomize-guestbook

echo "==> Checking ArgoCD applications"
kubectl get applications -n argocd

echo "==> Checking guestbook workloads"
kubectl get all -n guestbook

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
echo "Default path: the healthy guestbook Applications should be ready."
echo "If you added --with-contrast, expect extra brownfield fixtures and possibly failing contrast objects."
