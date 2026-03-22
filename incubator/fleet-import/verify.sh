#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

"$SCRIPT_DIR/setup.sh" >/dev/null

normalize() {
  jq -S '
    .clusters |= sort_by(.name) |
    .clusters[].import.suggestion.units |= sort_by(.slug) |
    .summary.byApp |= with_entries(.value |= sort) |
    .proposal.units |= sort_by(.slug)
  ' "$1"
}

normalize "$SCRIPT_DIR/expected-output/fleet-summary.json" > "$TMP_DIR/expected.json"
normalize "$SCRIPT_DIR/sample-output/fleet-summary.json" > "$TMP_DIR/actual.json"

diff -u "$TMP_DIR/expected.json" "$TMP_DIR/actual.json"
echo "Fleet aggregation output matches expected output"
