#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
FIXTURE="$SCRIPT_DIR/fixtures/watch-demo.yaml"
VAR_DIR="$SCRIPT_DIR/var"
EXPLAIN=0
EXPLAIN_JSON=0
CLUSTER_NAME="${WATCH_WEBHOOK_CLUSTER_NAME:-watch-webhook}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"
PORT="${WATCH_WEBHOOK_PORT:-8787}"
RECEIVER_PATH="/events"
RECEIVER_PID=""

cleanup_receiver() {
  if [[ -n "${RECEIVER_PID:-}" ]] && kill -0 "$RECEIVER_PID" 2>/dev/null; then
    kill "$RECEIVER_PID" 2>/dev/null || true
    wait "$RECEIVER_PID" 2>/dev/null || true
  fi
}
trap cleanup_receiver EXIT

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain) EXPLAIN=1 ;;
    --explain-json) EXPLAIN_JSON=1 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
  shift
done

if [[ $EXPLAIN -eq 1 ]]; then
  cat <<TEXT
This example will:
- create a local kind cluster named $CLUSTER_NAME
- apply fixtures/watch-demo.yaml into namespace watch-demo
- start a local webhook receiver on 127.0.0.1:$PORT
- run cub-scout watch --webhook http://127.0.0.1:$PORT$RECEIVER_PATH --once --namespace watch-demo
- write captured events and command logs under sample-output/
- not write ConfigHub state
TEXT
  exit 0
fi

if [[ $EXPLAIN_JSON -eq 1 ]]; then
  jq -n \
    --arg example "watch-webhook" \
    --arg clusterType "kind" \
    --arg clusterName "$CLUSTER_NAME" \
    --arg webhookURL "http://127.0.0.1:$PORT$RECEIVER_PATH" \
    '{example: $example, mutatesConfighub: false, mutatesLiveInfrastructure: true, clusterType: $clusterType, clusterName: $clusterName, webhookURL: $webhookURL}'
  exit 0
fi

command -v kind >/dev/null
command -v kubectl >/dev/null
command -v python3 >/dev/null
command -v cub-scout >/dev/null

mkdir -p "$VAR_DIR"
mkdir -p "$OUTPUT_DIR"
: > "$OUTPUT_DIR/webhook-events.jsonl"
: > "$OUTPUT_DIR/webhook.stdout.log"
: > "$OUTPUT_DIR/webhook.stderr.log"
: > "$OUTPUT_DIR/watch.stdout.log"
: > "$OUTPUT_DIR/watch.stderr.log"

export KUBECONFIG="$KUBECONFIG_PATH"
if ! kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind create cluster --name "$CLUSTER_NAME" --kubeconfig "$KUBECONFIG_PATH"
fi
kubectl config use-context "kind-$CLUSTER_NAME" >/dev/null
kubectl apply -f "$FIXTURE" >/dev/null
kubectl rollout status deployment/watch-demo -n watch-demo --timeout=120s >/dev/null

python3 "$SCRIPT_DIR/mock_webhook.py" --port "$PORT" --output "$OUTPUT_DIR/webhook-events.jsonl" >"$OUTPUT_DIR/webhook.stdout.log" 2>"$OUTPUT_DIR/webhook.stderr.log" &
RECEIVER_PID=$!
sleep 1

cub-scout watch --webhook "http://127.0.0.1:$PORT$RECEIVER_PATH" --once --namespace watch-demo >"$OUTPUT_DIR/watch.stdout.log" 2>"$OUTPUT_DIR/watch.stderr.log"

echo "Saved webhook events to: $OUTPUT_DIR/webhook-events.jsonl"
