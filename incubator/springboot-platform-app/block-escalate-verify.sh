#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROUTES_YAML="$ROOT_DIR/operational/field-routes.yaml"
RUNTIME_POLICY="$ROOT_DIR/upstream/platform/runtime-policy.yaml"

required_files=(
  "$ROOT_DIR/block-escalate.sh"
  "$ROOT_DIR/block-escalate-verify.sh"
  "$ROUTES_YAML"
  "$RUNTIME_POLICY"
)

for file in "${required_files[@]}"; do
  [[ -f "$file" ]] || {
    echo "missing required file: $file" >&2
    exit 1
  }
done

jq -e '.proof_type == "block-escalate-boundary"' < <("$ROOT_DIR/block-escalate.sh" --explain-json) >/dev/null
jq -e '.current_status == "not_proven"' < <("$ROOT_DIR/block-escalate.sh" --explain-json) >/dev/null

grep -q 'match: spring.datasource.\*' "$ROUTES_YAML"
grep -q 'defaultAction: generator-owned' "$ROUTES_YAML"
grep -q 'managedDatasource:' "$RUNTIME_POLICY"

attempt_output="$("$ROOT_DIR/block-escalate.sh" --render-attempt)"
grep -q -- '--dry-run' <<<"$attempt_output"
grep -q 'SPRING_DATASOURCE_URL=' <<<"$attempt_output"
grep -q 'block or escalated' <<<"$attempt_output" || grep -q 'blocked or escalated' <<<"$attempt_output"

echo "ok: springboot-platform-app block-escalate bundle is consistent"
