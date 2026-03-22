#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

if [[ ! -f "$OUTPUT_DIR/map.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

kubectl get deployment -n detectors-demo payments-api >/dev/null
kubectl get deployment -n detectors-demo infra-ui >/dev/null
kubectl get deployment -n detectors-demo debug-tool >/dev/null

jq -e 'map(select(.name == "payments-api" and .owner == "Internal Platform")) | length == 1' "$OUTPUT_DIR/map.json" >/dev/null
jq -e 'map(select(.name == "infra-ui" and .owner == "Pulumi")) | length == 1' "$OUTPUT_DIR/map.json" >/dev/null
jq -e 'map(select(.name == "debug-tool" and .owner == "Native")) | length == 1' "$OUTPUT_DIR/map.json" >/dev/null
jq -e '.owner == "Unknown - no recognized ownership labels found"' "$OUTPUT_DIR/payments-api.explain.json" >/dev/null
jq -e '.owner == "Unknown - no recognized ownership labels found"' "$OUTPUT_DIR/infra-ui.explain.json" >/dev/null
jq -e '.summary.ownerType == "Native"' "$OUTPUT_DIR/payments-api.trace.json" >/dev/null

echo "Custom ownership detector map checks passed and current explain/trace limitation captured"
