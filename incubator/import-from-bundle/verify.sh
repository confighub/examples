#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

"$SCRIPT_DIR/setup.sh" >/dev/null

actual="$SCRIPT_DIR/sample-output/suggestion.json"
expected="$SCRIPT_DIR/expected-output/suggestion.json"

actual_norm="$TMP_DIR/actual.json"
expected_norm="$TMP_DIR/expected.json"

jq '(.evidence.bundlePath) = "BUNDLE_PATH"' "$actual" > "$actual_norm"
jq '(.evidence.bundlePath) = "BUNDLE_PATH"' "$expected" > "$expected_norm"

diff -u "$expected_norm" "$actual_norm"
echo "Dry-run import proposal matches expected output"
