#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${APPTIQUE_ARGO_APPSET_CLUSTER_NAME:-apptique-argo-applicationset}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

require_cmd kubectl
export KUBECONFIG="$KUBECONFIG_PATH"
if [[ ! -f "$KUBECONFIG_PATH" ]]; then
  echo "Missing kubeconfig at $KUBECONFIG_PATH. Run ./setup.sh first." >&2
  exit 1
fi

kubectl get applicationset -n argocd apptique >/dev/null
kubectl get application -n argocd apptique-dev >/dev/null
kubectl get application -n argocd apptique-prod >/dev/null
kubectl get namespace apptique-dev >/dev/null
kubectl get namespace apptique-prod >/dev/null
kubectl get deployment -n apptique-dev frontend >/dev/null
kubectl get deployment -n apptique-prod frontend >/dev/null
kubectl rollout status deployment/frontend -n apptique-dev --timeout=180s >/dev/null
kubectl rollout status deployment/frontend -n apptique-prod --timeout=180s >/dev/null
kubectl get service -n apptique-dev frontend >/dev/null
kubectl get service -n apptique-prod frontend >/dev/null

if command -v cub-scout >/dev/null 2>&1; then
  cub-scout map list -q "owner=ArgoCD" >/dev/null 2>&1 || true
  cub-scout trace deployment/frontend -n apptique-dev >/dev/null 2>&1 || true
fi

echo "All apptique-argo-applicationset checks passed."
