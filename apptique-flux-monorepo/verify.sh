#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${APPTIQUE_FLUX_CLUSTER_NAME:-apptique-flux-monorepo}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"
WITH_PROD=0

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-prod)
      WITH_PROD=1
      shift
      ;;
    -h|--help)
      echo "Usage: ./verify.sh [--with-prod]" >&2
      exit 0
      ;;
    *)
      echo "Unexpected argument: $1" >&2
      exit 1
      ;;
  esac
done

require_cmd kubectl
export KUBECONFIG="$KUBECONFIG_PATH"
if [[ ! -f "$KUBECONFIG_PATH" ]]; then
  echo "Missing kubeconfig at $KUBECONFIG_PATH. Run ./setup.sh first." >&2
  exit 1
fi

kubectl get gitrepository -n flux-system apptique-examples >/dev/null
kubectl get kustomization -n flux-system apptique-dev >/dev/null
kubectl get namespace apptique-dev >/dev/null
kubectl get deployment -n apptique-dev frontend >/dev/null
kubectl rollout status deployment/frontend -n apptique-dev --timeout=180s >/dev/null
kubectl get service -n apptique-dev frontend >/dev/null

if [[ "$WITH_PROD" -eq 1 ]]; then
  kubectl get kustomization -n flux-system apptique-prod >/dev/null
  kubectl get namespace apptique-prod >/dev/null
  kubectl get deployment -n apptique-prod frontend >/dev/null
  kubectl rollout status deployment/frontend -n apptique-prod --timeout=180s >/dev/null
  kubectl get service -n apptique-prod frontend >/dev/null
fi

if command -v flux >/dev/null 2>&1; then
  flux get sources git -A >/dev/null
  flux get kustomizations -A >/dev/null
fi

if command -v cub-scout >/dev/null 2>&1; then
  cub-scout map list -q "namespace=apptique-*" >/dev/null 2>&1 || true
  cub-scout trace deployment/frontend -n apptique-dev >/dev/null 2>&1 || true
fi

echo "All apptique-flux-monorepo checks passed."
