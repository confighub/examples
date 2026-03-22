#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

if [[ ! -f "$OUTPUT_DIR/orphans.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

kubectl get deployment -n legacy-apps legacy-prometheus >/dev/null
kubectl get deployment -n temp-testing debug-nginx >/dev/null
kubectl get deployment -n temp-testing debug-busybox >/dev/null
kubectl get deployment -n default hotfix-worker >/dev/null
kubectl get configmap -n default manual-override >/dev/null
kubectl get secret -n default manual-api-key >/dev/null
kubectl get cronjob -n default manual-cleanup >/dev/null

jq -e 'map(select(.kind == "Deployment" and .name == "legacy-prometheus" and .namespace == "legacy-apps" and .owner == "Native")) | length == 1' "$OUTPUT_DIR/orphans.json" >/dev/null
jq -e 'map(select(.kind == "Deployment" and .name == "debug-nginx" and .namespace == "temp-testing" and .owner == "Native")) | length == 1' "$OUTPUT_DIR/orphans.json" >/dev/null
jq -e 'map(select(.kind == "Deployment" and .name == "debug-busybox" and .namespace == "temp-testing" and .owner == "Native")) | length == 1' "$OUTPUT_DIR/orphans.json" >/dev/null
jq -e 'map(select(.kind == "Deployment" and .name == "hotfix-worker" and .namespace == "default" and .owner == "Native")) | length == 1' "$OUTPUT_DIR/orphans.json" >/dev/null
jq -e 'map(select(.name == "manual-override" and .namespace == "default" and .owner == "Native")) | length == 1' "$OUTPUT_DIR/orphans.json" >/dev/null
jq -e 'map(select(.name == "manual-api-key" and .namespace == "default" and .owner == "Native")) | length == 1' "$OUTPUT_DIR/orphans.json" >/dev/null

if [[ -s "$OUTPUT_DIR/debug-nginx.trace.json" ]]; then
  jq -e '.command == "trace"' "$OUTPUT_DIR/debug-nginx.trace.json" >/dev/null
  jq -e '.target.kind == "Deployment" and .target.namespace == "temp-testing" and .target.name == "debug-nginx"' "$OUTPUT_DIR/debug-nginx.trace.json" >/dev/null
fi

echo "Orphan inventory checks passed"
