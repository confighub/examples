#!/usr/bin/env bash
# Compare inventory-api config across dev, stage, prod in ConfigHub.
#
# Shows a side-by-side table of key fields with divergence markers.
# Fields mutated in ConfigHub (diverging from upstream default) are marked *.
#
# Usage:
#   ./confighub-compare.sh              # Table output
#   ./confighub-compare.sh --explain    # What this script does (read-only)
#   ./confighub-compare.sh --json       # Machine-readable output
#
# Requires: cub, jq, python3
# Read-only: never mutates ConfigHub.

set -euo pipefail

CUB="${CUB:-cub}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ENVS=(dev stage prod)

# ── explain ──────────────────────────────────────────────────────────

show_explain() {
  cat <<'EOF'
confighub-compare: springboot-platform-app

Compares inventory-api operational config across dev, stage, and prod.

What it shows:
- Key deployment and config fields side by side
- Divergence markers (*) for fields whose live value differs from
  the upstream default for that environment
- Field route classification (mutable-in-ch / lift-upstream / generator-owned)

Read-only: this script fetches unit data but never mutates ConfigHub.

Commands used:
- cub unit get --space <space> --data-only --json inventory-api
EOF
}

if [[ "${1:-}" == "--explain" ]]; then
  show_explain
  exit 0
fi

command -v jq >/dev/null 2>&1 || { echo "error: jq not found." >&2; exit 1; }

# ── field model (portable JSON, no bash associative arrays) ──────────

FIELD_MODEL=$(cat <<'ENDJSON'
[
  {
    "label": "SPRING_PROFILES_ACTIVE",
    "display": "SPRING_PROFILES_ACTIVE",
    "route": "-",
    "upstream": { "dev": "default", "stage": "stage", "prod": "prod" },
    "extract": "env"
  },
  {
    "label": "reservationMode",
    "display": "feature.inventory.reservationMode",
    "route": "mutable-in-ch",
    "upstream": { "dev": "optimistic", "stage": "strict", "prod": "strict" },
    "extract": "configmap-grep"
  },
  {
    "label": "CACHE_BACKEND",
    "display": "CACHE_BACKEND",
    "route": "lift-upstream",
    "upstream": { "dev": "none", "stage": "none", "prod": "none" },
    "extract": "env"
  },
  {
    "label": "replicas",
    "display": "replicas",
    "route": "-",
    "upstream": { "dev": "1", "stage": "2", "prod": "3" },
    "extract": "replicas"
  },
  {
    "label": "containerPort",
    "display": "containerPort",
    "route": "-",
    "upstream": { "dev": "8080", "stage": "8080", "prod": "8081" },
    "extract": "port"
  },
  {
    "label": "SPRING_DATASOURCE_URL",
    "display": "SPRING_DATASOURCE_URL",
    "route": "generator-owned",
    "upstream": {
      "dev": "jdbc:postgresql://postgres.platform.svc:5432/inventory",
      "stage": "jdbc:postgresql://postgres.platform.svc:5432/inventory",
      "prod": "jdbc:postgresql://postgres.platform.svc:5432/inventory"
    },
    "extract": "env"
  }
]
ENDJSON
)

num_fields=$(echo "$FIELD_MODEL" | jq 'length')

# ── extract a field value from unit data JSON ────────────────────────

extract_value() {
  local data="$1" extract_type="$2" field_label="$3"
  case "$extract_type" in
    env)
      echo "$data" | jq -r --arg n "$field_label" '
        [.[] | select(.kind == "Deployment")
         | .spec.template.spec.containers[]?.env[]?
         | select(.name == $n) | .value] | first // "-"
      ' 2>/dev/null
      ;;
    configmap-grep)
      local val
      val=$(echo "$data" | jq -r '
        [.[] | select(.kind == "ConfigMap")
         | .data["application.yaml"] // empty] | first // empty
      ' 2>/dev/null | grep "$field_label" | sed 's/.*: *//' || echo "-")
      # Check env var override too
      local env_name
      env_name=$(echo "FEATURE_INVENTORY_RESERVATIONMODE")
      local env_val
      env_val=$(echo "$data" | jq -r --arg n "$env_name" '
        [.[] | select(.kind == "Deployment")
         | .spec.template.spec.containers[]?.env[]?
         | select(.name == $n) | .value] | first // empty
      ' 2>/dev/null)
      if [[ -n "$env_val" ]]; then val="$env_val"; fi
      echo "${val:-"-"}"
      ;;
    replicas)
      echo "$data" | jq -r '[.[] | select(.kind == "Deployment") | .spec.replicas] | first // "-"' 2>/dev/null
      ;;
    port)
      echo "$data" | jq -r '[.[] | select(.kind == "Deployment") | .spec.template.spec.containers[0].ports[0].containerPort] | first // "-"' 2>/dev/null
      ;;
    *)
      echo "-"
      ;;
  esac
}

# ── fetch unit data per environment ──────────────────────────────────

echo "Fetching unit data from ConfigHub..."
echo ""

# Build result as JSON array
RESULT="[]"

for env in "${ENVS[@]}"; do
  data=""
  if command -v "${CUB}" >/dev/null 2>&1; then
    data=$(${CUB} unit get --space "inventory-api-${env}" --data-only --json inventory-api 2>/dev/null || true)
  fi

  if [[ -z "$data" || "$data" == "null" ]]; then
    yaml="${SCRIPT_DIR}/confighub/inventory-api-${env}.yaml"
    if [[ -f "$yaml" ]]; then
      data=$(python3 -c "
import sys, json, yaml
with open('${yaml}') as f:
    docs = list(yaml.safe_load_all(f))
json.dump(docs, sys.stdout)
" 2>/dev/null || echo "[]")
      echo "  ${env}: using fixture fallback"
    else
      echo "  ${env}: no data available"
      continue
    fi
  else
    echo "  ${env}: fetched from ConfigHub"
  fi

  for ((i=0; i<num_fields; i++)); do
    label=$(echo "$FIELD_MODEL" | jq -r ".[$i].label")
    extract=$(echo "$FIELD_MODEL" | jq -r ".[$i].extract")
    upstream=$(echo "$FIELD_MODEL" | jq -r --arg e "$env" ".[$i].upstream[\$e]")

    val=$(extract_value "$data" "$extract" "$label")
    val="${val:-"-"}"

    diverged="false"
    if [[ -n "$upstream" && "$val" != "$upstream" && "$val" != "-" ]]; then
      diverged="true"
    fi

    RESULT=$(echo "$RESULT" | jq \
      --arg env "$env" --argjson idx "$i" \
      --arg val "$val" --argjson div "$diverged" \
      '. + [{ env: $env, fieldIndex: $idx, value: $val, diverged: $div }]')
  done
done

echo ""

# ── JSON output ──────────────────────────────────────────────────────

if [[ "${1:-}" == "--json" ]]; then
  # Restructure into a cleaner format
  output="{}"
  for ((i=0; i<num_fields; i++)); do
    display=$(echo "$FIELD_MODEL" | jq -r ".[$i].display")
    route=$(echo "$FIELD_MODEL" | jq -r ".[$i].route")
    field_obj=$(echo "$RESULT" | jq --argjson idx "$i" --arg d "$display" --arg r "$route" '
      { display: $d, route: $r, values:
        [.[] | select(.fieldIndex == $idx)] | reduce .[] as $item ({}; .[$item.env] = { value: $item.value, diverged: $item.diverged })
      }
    ')
    output=$(echo "$output" | jq --arg d "$display" --argjson obj "$field_obj" '.[$d] = $obj')
  done
  echo "$output" | jq .
  exit 0
fi

# ── table output ─────────────────────────────────────────────────────

printf "%-38s %-15s %-15s %-15s %s\n" "Field" "dev" "stage" "prod" "Route"
printf "%-38s %-15s %-15s %-15s %s\n" \
  "──────────────────────────────────────" \
  "───────────────" "───────────────" "───────────────" \
  "───────────────"

for ((i=0; i<num_fields; i++)); do
  display=$(echo "$FIELD_MODEL" | jq -r ".[$i].display")
  route=$(echo "$FIELD_MODEL" | jq -r ".[$i].route")

  vals=()
  for env in "${ENVS[@]}"; do
    entry=$(echo "$RESULT" | jq -r --arg e "$env" --argjson idx "$i" \
      '[.[] | select(.env == $e and .fieldIndex == $idx)] | first // { value: "-", diverged: false }')
    val=$(echo "$entry" | jq -r '.value')
    div=$(echo "$entry" | jq -r '.diverged')

    # Truncate long values
    if [[ ${#val} -gt 13 ]]; then val="${val:0:12}…"; fi
    if [[ "$div" == "true" ]]; then val="${val}*"; fi
    vals+=("$val")
  done

  printf "%-38s %-15s %-15s %-15s %s\n" "$display" "${vals[0]}" "${vals[1]}" "${vals[2]}" "$route"
done

echo ""
echo "* = value diverges from upstream default (mutated in ConfigHub or overridden)"
echo ""
echo "Route legend:"
echo "  mutable-in-ch   — safe to edit directly in ConfigHub"
echo "  lift-upstream    — route durable change back to source repo"
echo "  generator-owned  — blocked: platform-engineering owned"
