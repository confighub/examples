#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

if [[ ! -f "$OUTPUT_DIR/summary-list-all.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

jq -e '.count == 4' "$OUTPUT_DIR/summary-list-all.json" >/dev/null
jq -e '.entries | length == 4' "$OUTPUT_DIR/summary-list-all.json" >/dev/null
jq -e '.count == 2' "$OUTPUT_DIR/summary-list-kind-dev-prod.json" >/dev/null
jq -e '[.entries[].type] | sort == ["gitops-status","scan"]' "$OUTPUT_DIR/summary-list-kind-dev-prod.json" >/dev/null
jq -e '.text and (.blocks | length > 0)' "$OUTPUT_DIR/summary-slack-dry-run.json" >/dev/null
jq -e '.text | contains("Connected digest")' "$OUTPUT_DIR/summary-slack-dry-run.json" >/dev/null
jq -e '.blocks[].text?.text? // .blocks[].fields[]?.text? | strings' "$OUTPUT_DIR/summary-slack-dry-run.json" >/dev/null
jq -e '.blocks | tostring | contains("kind-dev") and contains("cub-scout summary list")' "$OUTPUT_DIR/summary-slack-dry-run.json" >/dev/null

echo "Connected summary storage checks passed"
