#!/usr/bin/env bash
# Show only the fields that differ between two environments.
#
# Platform engineers constantly ask "what's different between dev and prod?"
# This script answers that directly — no noise, just differences.
#
# Usage:
#   ./diff.sh dev prod       # Show differences between dev and prod
#   ./diff.sh dev stage      # Show differences between dev and stage
#   ./diff.sh stage prod     # Show differences between stage and prod
#   ./diff.sh --json dev prod  # Machine-readable diff
#   ./diff.sh --explain      # What this does

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPRING_DIR="${SCRIPT_DIR}/../springboot-platform-app"

JSON_MODE=false
if [[ "${1:-}" == "--json" ]]; then
  JSON_MODE=true
  shift
fi

if [[ "${1:-}" == "--explain" ]]; then
  cat <<'EOF'
platform-write-api diff

Shows only the fields that differ between two environments.
Uses the same data sources as compare.sh (ConfigHub live or fixtures).

  ./diff.sh dev prod    → differences between dev and prod
  ./diff.sh --json dev prod → machine-readable

This is the quickest way to answer "what's different between dev and prod?"
EOF
  exit 0
fi

ENV_A="${1:-}"
ENV_B="${2:-}"

if [[ -z "$ENV_A" || -z "$ENV_B" ]]; then
  echo "Usage: $0 [--json] <env1> <env2>" >&2
  echo "  Environments: dev, stage, prod" >&2
  echo "" >&2
  echo "Examples:" >&2
  echo "  $0 dev prod" >&2
  echo "  $0 --json dev prod" >&2
  exit 2
fi

for env in "$ENV_A" "$ENV_B"; do
  case "$env" in
    dev|stage|prod) ;;
    *) echo "error: unknown environment '$env'. Use: dev, stage, prod" >&2; exit 2 ;;
  esac
done

if [[ "$ENV_A" == "$ENV_B" ]]; then
  echo "error: comparing '$ENV_A' to itself — nothing to diff" >&2
  exit 2
fi

# --- Field definitions per environment ---
# These match the operational fixtures and the Spring Boot app config.
# The canonical source is the configmap.yaml (profile-specific) and deployment.yaml.

get_field_value() {
  local env="$1"
  local field="$2"

  # Try ConfigHub live first
  CUB="${CUB:-cub}"
  if command -v "${CUB}" >/dev/null 2>&1; then
    local space="inventory-api-${env}"
    local data
    data=$("${CUB}" unit get --space "${space}" --data-only --json inventory-api 2>/dev/null) || true
    if [[ -n "$data" && "$data" != "null" && "$data" != "[]" ]]; then
      # Extract from ConfigHub data
      local val
      case "$field" in
        replicas)
          val=$(echo "$data" | jq -r '[.[] | select(.kind == "Deployment") | .spec.replicas] | first // empty' 2>/dev/null)
          ;;
        containerPort)
          val=$(echo "$data" | jq -r '[.[] | select(.kind == "Deployment") | .spec.template.spec.containers[0].ports[0].containerPort] | first // empty' 2>/dev/null)
          ;;
        SPRING_PROFILES_ACTIVE|SPRING_DATASOURCE_URL|CACHE_BACKEND|FEATURE_INVENTORY_RESERVATIONMODE)
          val=$(echo "$data" | jq -r --arg f "$field" '[.[] | select(.kind == "Deployment") | .spec.template.spec.containers[0].env[]? | select(.name == $f) | .value] | first // empty' 2>/dev/null)
          ;;
        *)
          val=""
          ;;
      esac
      if [[ -n "$val" ]]; then
        echo "$val"
        return 0
      fi
    fi
  fi

  # Fall back to fixture-derived defaults
  case "${field}:${env}" in
    # feature.inventory.reservationMode
    "reservationMode:dev")     echo "optimistic" ;;
    "reservationMode:stage")   echo "cautious" ;;
    "reservationMode:prod")    echo "strict" ;;
    # spring.cache.type
    "cacheType:dev")           echo "none" ;;
    "cacheType:stage")         echo "none" ;;
    "cacheType:prod")          echo "none" ;;
    # spring.datasource.url
    "datasourceUrl:dev")       echo "jdbc:h2:mem:testdb" ;;
    "datasourceUrl:stage")     echo "jdbc:postgresql://pg-stage:5432/inventory" ;;
    "datasourceUrl:prod")      echo "jdbc:postgresql://pg-prod:5432/inventory" ;;
    # SPRING_PROFILES_ACTIVE
    "profiles:dev")            echo "default" ;;
    "profiles:stage")          echo "stage" ;;
    "profiles:prod")           echo "prod" ;;
    # replicas
    "replicas:dev")            echo "1" ;;
    "replicas:stage")          echo "2" ;;
    "replicas:prod")           echo "3" ;;
    # containerPort
    "containerPort:dev")       echo "8080" ;;
    "containerPort:stage")     echo "8080" ;;
    "containerPort:prod")      echo "8081" ;;
    # environment label
    "environment:dev")         echo "dev" ;;
    "environment:stage")       echo "stage" ;;
    "environment:prod")        echo "prod" ;;
    *)                         echo "—" ;;
  esac
}

# Field list with display names and route info
FIELDS=(
  "reservationMode|feature.inventory.reservationMode|mutable-in-ch"
  "cacheType|spring.cache.type|lift-upstream"
  "datasourceUrl|spring.datasource.url|generator-owned"
  "profiles|SPRING_PROFILES_ACTIVE|generator-owned"
  "replicas|replicas|mutable-in-ch"
  "containerPort|containerPort|generator-owned"
  "environment|inventory.environment|generator-owned"
)

diffs=()
diff_json="["

for entry in "${FIELDS[@]}"; do
  IFS='|' read -r key display route <<< "$entry"
  val_a=$(get_field_value "$ENV_A" "$key")
  val_b=$(get_field_value "$ENV_B" "$key")
  if [[ "$val_a" != "$val_b" ]]; then
    diffs+=("${display}|${val_a}|${val_b}|${route}")
    if [[ "$diff_json" != "[" ]]; then
      diff_json+=","
    fi
    diff_json+=$(printf '{"field":"%s","%s":"%s","%s":"%s","route":"%s"}' \
      "$display" "$ENV_A" "$val_a" "$ENV_B" "$val_b" "$route")
  fi
done

diff_json+="]"

if $JSON_MODE; then
  cat <<JSON
{
  "compare": ["${ENV_A}", "${ENV_B}"],
  "totalFields": ${#FIELDS[@]},
  "differencesCount": ${#diffs[@]},
  "differences": ${diff_json}
}
JSON
  exit 0
fi

# Pretty print
echo "=== Diff: ${ENV_A} vs ${ENV_B} ==="
echo ""

if [[ ${#diffs[@]} -eq 0 ]]; then
  echo "  No differences found."
  echo ""
  echo "  These environments have identical config."
else
  # Header
  header_a=$(echo "$ENV_A" | tr '[:lower:]' '[:upper:]')
  header_b=$(echo "$ENV_B" | tr '[:lower:]' '[:upper:]')
  printf "  %-40s  %-20s  %-20s  %s\n" "FIELD" "$header_a" "$header_b" "ROUTE"
  printf "  %-40s  %-20s  %-20s  %s\n" \
    "────────────────────────────────────────" \
    "────────────────────" \
    "────────────────────" \
    "──────────────────"

  for d in "${diffs[@]}"; do
    IFS='|' read -r field val_a val_b route <<< "$d"
    # Truncate long values for display
    [[ ${#val_a} -gt 18 ]] && val_a="${val_a:0:15}..."
    [[ ${#val_b} -gt 18 ]] && val_b="${val_b:0:15}..."
    printf "  %-40s  %-20s  %-20s  %s\n" "$field" "$val_a" "$val_b" "$route"
  done

  echo ""
  echo "  ${#diffs[@]} of ${#FIELDS[@]} fields differ between ${ENV_A} and ${ENV_B}."
fi

echo ""
echo "Full comparison:  ./compare.sh"
echo "Field routes:     ./field-routes.sh ${ENV_B}"
