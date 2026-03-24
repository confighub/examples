#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${IMPORT_FROM_LIVE_CLUSTER_NAME:-import-from-live}"
VAR_DIR="$SCRIPT_DIR/var"
export KUBECONFIG="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

if docker ps -a --format '{{.Names}}' 2>/dev/null | grep -qx "${CLUSTER_NAME}-control-plane"; then
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
fi
rm -f "$SCRIPT_DIR"/sample-output/*.json "$SCRIPT_DIR"/sample-output/*.txt
rm -f "$KUBECONFIG"

echo "Deleted kind cluster: $CLUSTER_NAME"
echo "Cleared local sample output"
