#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURE="$SCRIPT_DIR/fixtures/payments-platform.json"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
EXPLAIN=0
EXPLAIN_JSON=0

run_app() {
  (cd "$SCRIPT_DIR" && go run ./cmd/payments-rbac "$@")
}

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example is offline. It runs the payments-rbac Go app over a
payments-platform fixture and writes local RBAC-manager-style outputs under
sample-output/.
EOF_USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain)
      EXPLAIN=1
      ;;
    --explain-json)
      EXPLAIN_JSON=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unexpected argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
  shift
done

if [[ "$EXPLAIN" -eq 1 ]]; then
  command -v go >/dev/null 2>&1 || {
    echo "Missing required command: go" >&2
    exit 1
  }
  run_app explain
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  command -v go >/dev/null 2>&1 || {
    echo "Missing required command: go" >&2
    exit 1
  }
  run_app explain-json
  exit 0
fi

command -v go >/dev/null 2>&1 || {
  echo "Missing required command: go" >&2
  exit 1
}

mkdir -p "$OUTPUT_DIR"

run_app component-map > "$OUTPUT_DIR/component-map.json"
run_app snapshot > "$OUTPUT_DIR/snapshot.json"
run_app who-can > "$OUTPUT_DIR/who-can-get-secrets-prod-us.json"
run_app findings > "$OUTPUT_DIR/findings.json"
run_app plan > "$OUTPUT_DIR/proposed-edit.json"

echo "Saved payments-platform RBAC outputs to: $OUTPUT_DIR"
