#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
CLUSTER_NAME="${FLUX_BOUTIQUE_CLUSTER_NAME:-flux-boutique}"
VAR_DIR="$SCRIPT_DIR/var"
export KUBECONFIG="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null
  echo "Deleted kind cluster: $CLUSTER_NAME"
fi

rm -f "$OUTPUT_DIR/map-list.json" "$OUTPUT_DIR/trace-payment.json" "$OUTPUT_DIR/flux-kustomizations.txt"
rm -f "$KUBECONFIG"
echo "Cleared local sample output"
