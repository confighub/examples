#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

rm -f \
  "$OUTPUT_DIR/summary-list-all.json" \
  "$OUTPUT_DIR/summary-list-kind-dev-prod.json" \
  "$OUTPUT_DIR/summary-slack-dry-run.json"

echo "Cleared local sample output"
