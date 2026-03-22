#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
CLUSTER_NAME="${WATCH_WEBHOOK_CLUSTER_NAME:-watch-webhook}"

if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null
  echo "Deleted kind cluster: $CLUSTER_NAME"
fi

rm -f \
  "$OUTPUT_DIR/webhook-events.jsonl" \
  "$OUTPUT_DIR/webhook.stdout.log" \
  "$OUTPUT_DIR/webhook.stderr.log" \
  "$OUTPUT_DIR/watch.stdout.log" \
  "$OUTPUT_DIR/watch.stderr.log"

echo "Cleared local sample output"
