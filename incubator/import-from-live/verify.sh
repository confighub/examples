#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
EXPECTED="$SCRIPT_DIR/expected-output/suggestion.json"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if [[ ! -f "$OUTPUT_DIR/suggestion.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

kubectl get application -n argocd >/dev/null
kubectl get deployment -n myapp-dev >/dev/null
kubectl get deployment -n myapp-staging >/dev/null
kubectl get deployment -n myapp-prod >/dev/null
kubectl get statefulset -n myapp-prod >/dev/null
kubectl get configmap -n myapp-prod debug-config >/dev/null

expected_norm="$TMP_DIR/expected.json"
actual_norm="$TMP_DIR/actual.json"

jq -S . "$EXPECTED" > "$expected_norm"
jq -S . "$OUTPUT_DIR/suggestion.json" > "$actual_norm"

diff -u "$expected_norm" "$actual_norm"
echo "Live import suggestion matches expected output"
