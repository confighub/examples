#!/usr/bin/env bash
# Show field-level mutation routing for order-api in ConfigHub.
#
# Reads operational/field-routes.yaml and annotates it with the current
# ConfigHub state for a given environment.
#
# Usage:
#   ./confighub-field-routes.sh              # Show routes for prod (default)
#   ./confighub-field-routes.sh dev          # Show routes for dev
#   ./confighub-field-routes.sh --explain    # What this script does
#   ./confighub-field-routes.sh --json       # Machine-readable output
#
# Read-only: never mutates ConfigHub.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CUB="${CUB:-cub}"
FIELD_ROUTES="${SCRIPT_DIR}/operational/field-routes.yaml"

# ── explain ──────────────────────────────────────────────────────────

show_explain() {
  cat <<'EOF'
confighub-field-routes: springboot-platform-app

Shows how each config field is classified for mutation routing.

For each field pattern in operational/field-routes.yaml, shows:
- the match pattern
- who owns it (app-team vs platform-engineering)
- what happens when someone tries to change it
- current value in ConfigHub for the selected environment

This is what the GUI should show as badges on each field in the
unit editor: can you edit this? should you route it upstream?
will it be blocked?

Read-only: fetches unit data but never mutates ConfigHub.
EOF
}

if [[ "${1:-}" == "--explain" ]]; then
  show_explain
  exit 0
fi

# ── parse args ───────────────────────────────────────────────────────

ENV="${1:-prod}"
JSON_MODE=false
if [[ "$ENV" == "--json" ]]; then
  JSON_MODE=true
  ENV="${2:-prod}"
fi
if [[ "${2:-}" == "--json" ]]; then
  JSON_MODE=true
fi

SPACE="order-api-${ENV}"

# ── field routes from YAML ───────────────────────────────────────────

if [[ ! -f "$FIELD_ROUTES" ]]; then
  echo "error: ${FIELD_ROUTES} not found" >&2
  exit 1
fi

# Parse routes using python3 + PyYAML (available on macOS)
ROUTES_JSON=$(python3 -c "
import sys, json, yaml
with open('${FIELD_ROUTES}') as f:
    data = yaml.safe_load(f)
json.dump(data.get('routes', []), sys.stdout)
" 2>/dev/null || echo "[]")

# ── decode unit data from cub JSON response ──────────────────────────
decode_unit_data() {
  local raw="$1"
  if echo "$raw" | jq -e 'type == "array"' >/dev/null 2>&1; then
    echo "$raw"
    return
  fi
  local b64
  b64=$(echo "$raw" | jq -r '.Unit.Data // empty' 2>/dev/null)
  if [[ -z "$b64" ]]; then
    echo "[]"
    return
  fi
  echo "$b64" | base64 -d 2>/dev/null | python3 -c "
import sys, json, yaml
docs = list(yaml.safe_load_all(sys.stdin))
json.dump([d for d in docs if d], sys.stdout)
" 2>/dev/null || echo "[]"
}

# ── current values from ConfigHub ────────────────────────────────────

RAW_DATA=""
if command -v "${CUB}" >/dev/null 2>&1; then
  RAW_DATA=$(${CUB} unit get --space "${SPACE}" --data-only --json order-api 2>/dev/null || true)
fi

if [[ -z "$RAW_DATA" || "$RAW_DATA" == "null" ]]; then
  # Fall back to fixture files
  yaml="${SCRIPT_DIR}/confighub/order-api-${ENV}.yaml"
  if [[ -f "$yaml" ]]; then
    UNIT_DATA=$(python3 -c "
import sys, json, yaml
with open('${yaml}') as f:
    docs = list(yaml.safe_load_all(f))
json.dump(docs, sys.stdout)
" 2>/dev/null || echo "[]")
  else
    UNIT_DATA="[]"
  fi
else
  UNIT_DATA=$(decode_unit_data "$RAW_DATA")
fi

# Extract a representative value for a field pattern
get_current_value() {
  local pattern="$1"
  local data="$2"
  [[ -z "$data" ]] && { echo "-"; return; }

  case "$pattern" in
    "feature.inventory.*")
      local val
      val=$(echo "$data" | jq -r '
        [.[] | select(.kind == "ConfigMap")
         | .data["application.yaml"] // empty] | first // empty
      ' 2>/dev/null | grep 'reservationMode' | sed 's/.*: *//' || echo "")
      # Check env var override
      local env_val
      env_val=$(echo "$data" | jq -r '
        [.[] | select(.kind == "Deployment")
         | .spec.template.spec.containers[]?.env[]?
         | select(.name == "FEATURE_INVENTORY_RESERVATIONMODE") | .value] | first // empty
      ' 2>/dev/null)
      if [[ -n "$env_val" ]]; then val="$env_val (env override)"; fi
      echo "${val:-"-"}"
      ;;
    "spring.cache.*")
      local val
      val=$(echo "$data" | jq -r '
        [.[] | select(.kind == "Deployment")
         | .spec.template.spec.containers[]?.env[]?
         | select(.name == "CACHE_BACKEND") | .value] | first // empty
      ' 2>/dev/null)
      echo "${val:-"not set"}"
      ;;
    "spring.datasource.*")
      local val
      val=$(echo "$data" | jq -r '
        [.[] | select(.kind == "Deployment")
         | .spec.template.spec.containers[]?.env[]?
         | select(.name == "SPRING_DATASOURCE_URL") | .value] | first // empty
      ' 2>/dev/null)
      if [[ ${#val} -gt 40 ]]; then val="${val:0:39}…"; fi
      echo "${val:-"-"}"
      ;;
    "securityContext.*")
      local val
      val=$(echo "$data" | jq -r '
        [.[] | select(.kind == "Deployment")
         | .spec.template.spec.containers[0].securityContext // empty] | first // empty
      ' 2>/dev/null)
      echo "${val:-"(not set in fixture)"}"
      ;;
    *)
      echo "-"
      ;;
  esac
}

# ── route symbol ─────────────────────────────────────────────────────

route_symbol() {
  case "$1" in
    mutable-in-ch)   echo "✓ safe to edit" ;;
    lift-upstream)   echo "↑ route to repo" ;;
    generator-owned) echo "✗ blocked" ;;
    *)               echo "? unknown" ;;
  esac
}

# ── JSON output ──────────────────────────────────────────────────────

if [[ "$JSON_MODE" == "true" ]]; then
  result="[]"
  num_routes=$(echo "$ROUTES_JSON" | jq 'length')
  for ((r=0; r<num_routes; r++)); do
    match=$(echo "$ROUTES_JSON" | jq -r ".[$r].match")
    owner=$(echo "$ROUTES_JSON" | jq -r ".[$r].owner")
    action=$(echo "$ROUTES_JSON" | jq -r ".[$r].defaultAction")
    reason=$(echo "$ROUTES_JSON" | jq -r ".[$r].reason")
    current=$(get_current_value "$match" "$UNIT_DATA")
    result=$(echo "$result" | jq \
      --arg m "$match" --arg o "$owner" --arg a "$action" \
      --arg r "$reason" --arg c "$current" --arg e "$ENV" \
      '. + [{ match: $m, owner: $o, action: $a, reason: $r, currentValue: $c, environment: $e }]')
  done
  echo "$result" | jq .
  exit 0
fi

# ── table output ─────────────────────────────────────────────────────

echo "Field routes for order-api-${ENV}:"
echo "────────────────────────────────────────────────────────────────────────"
echo ""

num_routes=$(echo "$ROUTES_JSON" | jq 'length')
for ((r=0; r<num_routes; r++)); do
  match=$(echo "$ROUTES_JSON" | jq -r ".[$r].match")
  owner=$(echo "$ROUTES_JSON" | jq -r ".[$r].owner")
  action=$(echo "$ROUTES_JSON" | jq -r ".[$r].defaultAction")
  reason=$(echo "$ROUTES_JSON" | jq -r ".[$r].reason")
  current=$(get_current_value "$match" "$UNIT_DATA")
  symbol=$(route_symbol "$action")

  printf "  %-25s → %-18s (%s)\n" "$match" "$action" "$owner"
  printf "    %s\n" "$symbol"
  printf "    current: %s\n" "$current"
  printf "    reason:  %s\n" "$reason"
  echo ""
done

echo "Legend:"
echo "  ✓ safe to edit    — mutable directly in ConfigHub"
echo "  ↑ route to repo   — durable change belongs upstream"
echo "  ✗ blocked         — platform-engineering owned, cannot diverge"
