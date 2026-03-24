#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${APPTIQUE_FLUX_CLUSTER_NAME:-apptique-flux-monorepo}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

cluster_exists() {
  docker ps -a --format '{{.Names}}' | grep -qx "${CLUSTER_NAME}-control-plane"
}

if command -v kind >/dev/null 2>&1 && command -v docker >/dev/null 2>&1; then
  if cluster_exists; then
    kind delete cluster --name "$CLUSTER_NAME"
  fi
fi

rm -f "$KUBECONFIG_PATH"

echo "Cleaned up apptique Flux monorepo cluster state."
