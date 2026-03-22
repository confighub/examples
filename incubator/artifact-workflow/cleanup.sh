#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

rm -f \
  "$OUTPUT_DIR/bundle-inspect.json" \
  "$OUTPUT_DIR/bundle-replay-drift.json" \
  "$OUTPUT_DIR/bundle-summarize.json"

echo "Cleared local sample output"
