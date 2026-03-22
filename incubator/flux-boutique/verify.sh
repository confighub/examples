#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${FLUX_BOUTIQUE_CLUSTER_NAME:-flux-boutique}"
export KUBECONFIG="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

if [[ ! -f "$OUTPUT_DIR/map-list.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

for name in frontend cart checkout payment shipping; do
  kubectl get deployment -n boutique "$name" >/dev/null
  jq -e --arg name "$name" 'map(select(.namespace == "boutique" and .kind == "Deployment" and .name == $name and .owner == "Flux")) | length == 1' "$OUTPUT_DIR/map-list.json" >/dev/null
done
jq -e '.target.kind == "Deployment" and .target.namespace == "boutique" and .target.name == "payment"' "$OUTPUT_DIR/trace-payment.json" >/dev/null
jq -e '.summary.ownerType == "Flux"' "$OUTPUT_DIR/trace-payment.json" >/dev/null

echo "Flux boutique checks passed"
