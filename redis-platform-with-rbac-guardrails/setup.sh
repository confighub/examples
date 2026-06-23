#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURE="$SCRIPT_DIR/fixtures/payments-platform.json"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example is offline. It reads a payments-platform fixture and writes local
RBAC-manager-style outputs under sample-output/.
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
  cat <<EOF_PLAN
This is a read-only setup plan for redis-platform-with-rbac-guardrails.
No ConfigHub state will be mutated.
No live infrastructure will be mutated.

The example models:
- Component: payments-platform
- Variants: base, dev, staging, prod-us, prod-eu
- Pieces: redis, payments-api, RBAC guardrails

When run normally, the script writes local files under:
- ${OUTPUT_DIR}

Those files show:
- the payments component map
- an RBAC snapshot
- who can get Secrets in payments-prod
- one visible RBAC finding
- a dry-run hardening edit
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg example "redis-platform-with-rbac-guardrails" \
    --arg component "payments-platform" \
    --arg fixture "fixtures/payments-platform.json" \
    '{
      example_name: $example,
      component: $component,
      mutates: false,
      mutates_confighub: false,
      mutates_live_infra: false,
      fixture: $fixture,
      outputs: [
        "sample-output/component-map.json",
        "sample-output/snapshot.json",
        "sample-output/who-can-get-secrets-prod-us.json",
        "sample-output/findings.json",
        "sample-output/proposed-edit.json"
      ]
    }'
  exit 0
fi

command -v jq >/dev/null 2>&1 || {
  echo "Missing required command: jq" >&2
  exit 1
}

mkdir -p "$OUTPUT_DIR"

jq '{component, description, variants, pieces}' "$FIXTURE" > "$OUTPUT_DIR/component-map.json"
jq '{
  component,
  variantCount: (.variants | length),
  pieceCount: (.pieces | length),
  unitCount: (.units | length),
  unitsByPiece: (.units | group_by(.piece) | map({piece: .[0].piece, count: length})),
  namespaces: ([.units[].namespace | select(. != "")] | unique)
}' "$FIXTURE" > "$OUTPUT_DIR/snapshot.json"
jq '.queries[] | select(.name == "who-can-get-secrets-prod-us")' "$FIXTURE" > "$OUTPUT_DIR/who-can-get-secrets-prod-us.json"
jq '{component, findings}' "$FIXTURE" > "$OUTPUT_DIR/findings.json"
jq '{component, proposedEdits}' "$FIXTURE" > "$OUTPUT_DIR/proposed-edit.json"

echo "Saved payments-platform RBAC outputs to: $OUTPUT_DIR"
