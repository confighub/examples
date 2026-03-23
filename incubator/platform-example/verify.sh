#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${PLATFORM_EXAMPLE_CLUSTER_NAME:-platform-example}"
export KUBECONFIG="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

if [[ ! -f "$OUTPUT_DIR/map-list.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

kubectl get deploy -n podinfo podinfo >/dev/null
kubectl get deploy -n default debug-nginx >/dev/null
kubectl get configmap -n default manual-config >/dev/null
jq -e 'map(select(.owner == "Flux" and .kind == "Deployment" and .namespace == "podinfo" and .name == "podinfo")) | length == 1' "$OUTPUT_DIR/map-list.json" >/dev/null
jq -e 'map(select(.owner == "Native" and .kind == "Deployment" and .namespace == "default" and .name == "debug-nginx")) | length == 1' "$OUTPUT_DIR/orphans.json" >/dev/null
jq -e '.target.kind == "Deployment" and .target.namespace == "podinfo" and .target.name == "podinfo"' "$OUTPUT_DIR/trace-podinfo.json" >/dev/null
jq -e '.summary.ownerType == "Flux"' "$OUTPUT_DIR/trace-podinfo.json" >/dev/null

echo "Platform example checks passed"
