#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
"$SCRIPT_DIR/setup.sh" >/dev/null

for name in dev-eshop prod-eshop prod-website; do
  jq 'del(.static.scannedAt)' "$SCRIPT_DIR/expected-output/${name}.scan.json" > "$TMP_DIR/${name}.expected.json"
  jq 'del(.static.scannedAt)' "$SCRIPT_DIR/sample-output/${name}.scan.json" > "$TMP_DIR/${name}.actual.json"
  diff -u "$TMP_DIR/${name}.expected.json" "$TMP_DIR/${name}.actual.json"
done

echo "Scan output matches expected output"
