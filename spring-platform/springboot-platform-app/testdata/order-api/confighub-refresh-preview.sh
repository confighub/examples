#!/usr/bin/env bash
# Preview what would happen if the generator re-rendered operational config.
#
# Compares current ConfigHub state against the upstream-rendered fixtures,
# through the lens of field-routes.yaml, and shows which fields would be:
#   PRESERVE — mutated in ConfigHub, survives refresh (mutable-in-ch)
#   REFRESH  — matches upstream or no local override
#   BLOCKED  — generator-owned, local divergence would be rejected
#
# Usage:
#   ./confighub-refresh-preview.sh              # Preview for prod (default)
#   ./confighub-refresh-preview.sh dev          # Preview for dev
#   ./confighub-refresh-preview.sh --explain    # What this does
#   ./confighub-refresh-preview.sh --json       # Machine-readable
#
# Read-only: never mutates ConfigHub.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CUB="${CUB:-cub}"

# ── explain ──────────────────────────────────────────────────────────

show_explain() {
  cat <<'EOF'
confighub-refresh-preview: springboot-platform-app

Simulates what would happen if the platform generator re-rendered
operational config and tried to refresh ConfigHub.

This is the key test for "merge, not overwrite":
- Fields classified as mutable-in-ch that have been changed locally
  should be PRESERVED (the local mutation wins).
- Fields that match upstream should be REFRESHED normally.
- Fields classified as generator-owned should never have local
  divergence; if they do, the refresh should REJECT the divergence.

This preview is a client-side simulation. Real refresh-survival
requires server-side merge support (not yet implemented).

Read-only: fetches data but never mutates ConfigHub.
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

command -v jq >/dev/null 2>&1 || { echo "error: jq not found." >&2; exit 1; }

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

# ── field model as JSON (avoids bash associative arrays for portability) ─

FIELD_MODEL=$(cat <<'ENDJSON'
[
  {
    "field": "SPRING_PROFILES_ACTIVE",
    "display": "SPRING_PROFILES_ACTIVE",
    "route": "infra",
    "upstream": { "dev": "default", "stage": "stage", "prod": "prod" }
  },
  {
    "field": "FEATURE_INVENTORY_RESERVATIONMODE",
    "display": "feature.inventory.reservationMode",
    "route": "mutable-in-ch",
    "upstream": { "dev": "optimistic", "stage": "strict", "prod": "strict" }
  },
  {
    "field": "CACHE_BACKEND",
    "display": "CACHE_BACKEND",
    "route": "lift-upstream",
    "upstream": { "dev": "none", "stage": "none", "prod": "none" }
  },
  {
    "field": "SPRING_DATASOURCE_URL",
    "display": "spring.datasource.url",
    "route": "generator-owned",
    "upstream": {
      "dev": "jdbc:postgresql://postgres.platform.svc:5432/inventory",
      "stage": "jdbc:postgresql://postgres.platform.svc:5432/inventory",
      "prod": "jdbc:postgresql://postgres.platform.svc:5432/inventory"
    }
  },
  {
    "field": "replicas",
    "display": "replicas",
    "route": "infra",
    "upstream": { "dev": "1", "stage": "2", "prod": "3" }
  }
]
ENDJSON
)

# ── fetch current ConfigHub values ───────────────────────────────────

RAW_DATA=""
if command -v "${CUB}" >/dev/null 2>&1; then
  RAW_DATA=$(${CUB} unit get --space "${SPACE}" --data-only --json order-api 2>/dev/null || true)
fi

if [[ -z "$RAW_DATA" || "$RAW_DATA" == "null" ]]; then
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

get_live_value() {
  local field_name="$1" data="$2"
  case "$field_name" in
    SPRING_PROFILES_ACTIVE|CACHE_BACKEND|SPRING_DATASOURCE_URL)
      echo "$data" | jq -r --arg n "$field_name" '
        [.[] | select(.kind == "Deployment")
         | .spec.template.spec.containers[]?.env[]?
         | select(.name == $n) | .value] | first // empty
      ' 2>/dev/null
      ;;
    FEATURE_INVENTORY_RESERVATIONMODE)
      local env_val
      env_val=$(echo "$data" | jq -r '
        [.[] | select(.kind == "Deployment")
         | .spec.template.spec.containers[]?.env[]?
         | select(.name == "FEATURE_INVENTORY_RESERVATIONMODE") | .value] | first // empty
      ' 2>/dev/null)
      if [[ -n "$env_val" ]]; then echo "$env_val"; return; fi
      # Fall back to ConfigMap
      echo "$data" | jq -r '
        [.[] | select(.kind == "ConfigMap")
         | .data["application.yaml"] // empty] | first // empty
      ' 2>/dev/null | grep 'reservationMode' | sed 's/.*: *//'
      ;;
    replicas)
      echo "$data" | jq -r '[.[] | select(.kind == "Deployment") | .spec.replicas] | first // empty' 2>/dev/null
      ;;
  esac
}

# ── compute refresh actions and build result JSON ────────────────────

num_fields=$(echo "$FIELD_MODEL" | jq 'length')
RESULT="[]"

for ((i=0; i<num_fields; i++)); do
  field_name=$(echo "$FIELD_MODEL" | jq -r ".[$i].field")
  display=$(echo "$FIELD_MODEL" | jq -r ".[$i].display")
  route=$(echo "$FIELD_MODEL" | jq -r ".[$i].route")
  upstream=$(echo "$FIELD_MODEL" | jq -r --arg e "$ENV" ".[$i].upstream[\$e]")

  live=$(get_live_value "$field_name" "$UNIT_DATA")
  live="${live:-"-"}"

  if [[ -z "$live" || "$live" == "-" ]]; then
    action="REFRESH"
    reason="no local value, upstream wins"
  elif [[ "$live" == "$upstream" ]]; then
    action="REFRESH"
    reason="matches upstream, no conflict"
  else
    case "$route" in
      mutable-in-ch)
        action="PRESERVE"
        reason="local override via mutable-in-ch route"
        ;;
      lift-upstream)
        action="REFRESH"
        reason="would refresh; durable change should be lifted upstream first"
        ;;
      generator-owned)
        action="REJECT"
        reason="local divergence on generator-owned field — should not exist"
        ;;
      *)
        action="REFRESH"
        reason="infrastructure field, upstream wins"
        ;;
    esac
  fi

  RESULT=$(echo "$RESULT" | jq \
    --arg f "$display" \
    --arg live "$live" \
    --arg upstream "$upstream" \
    --arg action "$action" \
    --arg reason "$reason" \
    --arg route "$route" \
    --arg env "$ENV" \
    '. + [{ field: $f, liveValue: $live, upstreamValue: $upstream, action: $action, reason: $reason, route: $route, environment: $env }]')
done

# ── JSON output ──────────────────────────────────────────────────────

if [[ "$JSON_MODE" == "true" ]]; then
  echo "$RESULT" | jq .
  exit 0
fi

# ── table output ─────────────────────────────────────────────────────

echo "Refresh preview for order-api-${ENV}:"
echo "What happens when the generator re-renders and tries to update ConfigHub?"
echo ""
echo "────────────────────────────────────────────────────────────────────────"

for ((i=0; i<num_fields; i++)); do
  display=$(echo "$RESULT" | jq -r ".[$i].field")
  live=$(echo "$RESULT" | jq -r ".[$i].liveValue")
  upstream=$(echo "$RESULT" | jq -r ".[$i].upstreamValue")
  action=$(echo "$RESULT" | jq -r ".[$i].action")
  reason=$(echo "$RESULT" | jq -r ".[$i].reason")

  # Truncate long values
  live_display="$live"
  upstream_display="$upstream"
  if [[ ${#live_display} -gt 45 ]]; then live_display="${live_display:0:44}…"; fi
  if [[ ${#upstream_display} -gt 45 ]]; then upstream_display="${upstream_display:0:44}…"; fi

  case "$action" in
    PRESERVE) symbol="PRESERVE" ;;
    REFRESH)  symbol="REFRESH " ;;
    REJECT)   symbol="REJECT  " ;;
    *)        symbol="UNKNOWN " ;;
  esac

  echo ""
  printf "  %-40s  %s\n" "$display" "$symbol"
  printf "    live:     %s\n" "$live_display"
  printf "    upstream: %s\n" "$upstream_display"
  printf "    reason:   %s\n" "$reason"
done

echo ""
echo "────────────────────────────────────────────────────────────────────────"
echo ""
echo "Legend:"
echo "  PRESERVE — local mutation wins; upstream refresh does not overwrite"
echo "  REFRESH  — upstream value accepted (no conflict or matches current)"
echo "  REJECT   — local divergence on a generator-owned field (should not exist)"
echo ""
echo "Note: This is a client-side simulation. Real refresh-survival requires"
echo "server-side policy-driven merge (not yet implemented in ConfigHub)."
