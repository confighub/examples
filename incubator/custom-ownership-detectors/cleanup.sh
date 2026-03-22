#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${CUSTOM_OWNERSHIP_CLUSTER_NAME:-custom-ownership-detectors}"
VAR_DIR="$SCRIPT_DIR/var"
export KUBECONFIG="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
rm -f "$SCRIPT_DIR"/sample-output/*.json "$SCRIPT_DIR"/sample-output/*.txt
rm -f "$KUBECONFIG"

echo "Deleted kind cluster: $CLUSTER_NAME"
echo "Cleared local sample output"
