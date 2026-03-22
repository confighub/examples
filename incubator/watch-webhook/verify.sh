#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

if [[ ! -f "$OUTPUT_DIR/webhook-events.jsonl" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

kubectl get deployment -n watch-demo watch-demo >/dev/null
kubectl get service -n watch-demo watch-demo >/dev/null

jq -e '.receivedAt and .path and .event.type and .event.resource.kind and .event.resource.name' "$OUTPUT_DIR/webhook-events.jsonl" >/dev/null
jq -e 'select(.event.type == "resource.discovered" and .event.resource.namespace == "watch-demo")' "$OUTPUT_DIR/webhook-events.jsonl" >/dev/null

echo "Watch webhook checks passed"
