#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

command -v jq >/dev/null 2>&1 || {
  echo "Missing required command: jq" >&2
  exit 1
}

if [[ ! -f "$OUTPUT_DIR/component-map.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

jq -e '.component == "payments-platform"' "$OUTPUT_DIR/component-map.json" >/dev/null
jq -e '.variants | map(.name) == ["base","dev","staging","prod-us","prod-eu"]' "$OUTPUT_DIR/component-map.json" >/dev/null
jq -e '.pieces | map(.name) | contains(["redis","payments-api","rbac-guardrails"])' "$OUTPUT_DIR/component-map.json" >/dev/null

jq -e '.unitCount == 7' "$OUTPUT_DIR/snapshot.json" >/dev/null
jq -e '.namespaces == ["payments-prod"]' "$OUTPUT_DIR/snapshot.json" >/dev/null
jq -e '.unitsByPiece | map(select(.piece == "redis" and .count == 3)) | length == 1' "$OUTPUT_DIR/snapshot.json" >/dev/null
jq -e '.unitsByPiece | map(select(.piece == "payments-api" and .count == 3)) | length == 1' "$OUTPUT_DIR/snapshot.json" >/dev/null

jq -e '.grants | length == 2' "$OUTPUT_DIR/who-can-get-secrets-prod-us.json" >/dev/null
jq -e '.grants | map(.subject) | contains(["ServiceAccount/payments-prod/redis","ServiceAccount/payments-prod/payments-api"])' "$OUTPUT_DIR/who-can-get-secrets-prod-us.json" >/dev/null
jq -e '.grants[] | select(.subject == "ServiceAccount/payments-prod/payments-api") | .scope == "all secrets in namespace"' "$OUTPUT_DIR/who-can-get-secrets-prod-us.json" >/dev/null

jq -e '.findings | length == 1' "$OUTPUT_DIR/findings.json" >/dev/null
jq -e '.findings[0].id == "payments-api-secret-list-prod-us"' "$OUTPUT_DIR/findings.json" >/dev/null
jq -e '.findings[0].betterPlace | contains("derived prod-us variant")' "$OUTPUT_DIR/findings.json" >/dev/null

jq -e '.proposedEdits | length == 1' "$OUTPUT_DIR/proposed-edit.json" >/dev/null
jq -e '.proposedEdits[0].mode == "dry-run"' "$OUTPUT_DIR/proposed-edit.json" >/dev/null
jq -e '.proposedEdits[0].humanReview == "required"' "$OUTPUT_DIR/proposed-edit.json" >/dev/null
jq -e '.proposedEdits[0].diff | map(.path) | contains(["rules[0].verbs","rules[0].resourceNames"])' "$OUTPUT_DIR/proposed-edit.json" >/dev/null

echo "Redis payments-platform RBAC guardrail checks passed"
