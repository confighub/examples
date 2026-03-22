#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

if [[ ! -f "$OUTPUT_DIR/hooks.json" || ! -f "$OUTPUT_DIR/lifecycle-scan.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

expected_hooks="$SCRIPT_DIR/expected-output/hooks.json"
expected_scan="$SCRIPT_DIR/expected-output/lifecycle-scan.json"

jq -S . "$OUTPUT_DIR/hooks.json" > "$TMP_DIR/hooks.actual.json"
diff -u "$expected_hooks" "$TMP_DIR/hooks.actual.json"

jq '(.lifecycleHazards.scannedAt) = "TIMESTAMP" | (.static.scannedAt) = "TIMESTAMP" | (.static.file) = "FIXTURE_PATH"' "$OUTPUT_DIR/lifecycle-scan.json" | jq -S . > "$TMP_DIR/lifecycle.actual.json"
diff -u "$expected_scan" "$TMP_DIR/lifecycle.actual.json"

echo "Lifecycle hazards outputs match expected output"
