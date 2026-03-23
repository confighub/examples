#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${APPTIQUE_ARGO_APPOFAPPS_CLUSTER_NAME:-apptique-argo-app-of-apps}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

if command -v kind >/dev/null 2>&1; then
  if kind get clusters | grep -qx "$CLUSTER_NAME"; then
    kind delete cluster --name "$CLUSTER_NAME"
  fi
fi

rm -f "$KUBECONFIG_PATH"

echo "Cleaned up apptique Argo app-of-apps cluster state."
