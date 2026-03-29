#!/bin/bash
# Verification wrapper for promotion-demo-data
#
# This script lives in incubator/ because the promotion-demo-data example
# is a stable example that shouldn't have incubator-style scripts added to it.
#
# Usage:
#   ./verify.sh                 # Run verification with text output
#   ./verify.sh --json          # Run verification with JSON output
#   ./verify.sh --explain       # Read-only preview
#   ./verify.sh --explain-json  # Machine-readable preview

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="${SCRIPT_DIR}/../../promotion-demo-data"

show_explain() {
  cat <<'EOF'
promotion-demo-data-verify

Read-only verification wrapper for the stable promotion-demo-data example.

What it checks:
- total demo space count
- platform-owned and prod space counts
- total unit count across app spaces
- label-query presence for a known app
- intentional version skew for eshop api images
- existence of a few key targets

What it reads:
- live ConfigHub spaces, units, targets, and image fields
- shared metadata from ../../promotion-demo-data/lib.sh

What it writes:
- nothing

Safe next steps:
  cd ../../promotion-demo-data && ./setup.sh
  cd ../incubator/promotion-demo-data-verify && ./verify.sh
  ./verify.sh --json
EOF
}

show_explain_json() {
  cat <<'EOF'
{
  "example_name": "promotion-demo-data-verify",
  "proof_type": "verification-wrapper",
  "mutates_confighub": false,
  "mutates_live_infra": false,
  "requires_cluster": false,
  "depends_on_example": "../../promotion-demo-data",
  "setup_command": "cd ../../promotion-demo-data && ./setup.sh",
  "cleanup_command": "cd ../../promotion-demo-data && ./cleanup.sh",
  "checks": [
    { "name": "total_demo_spaces", "comparison": "gte", "expected": 49 },
    { "name": "platform_owned_spaces", "comparison": "gte", "expected": 8 },
    { "name": "prod_spaces", "comparison": "gte", "expected": 12 },
    { "name": "total_units_across_app_spaces", "comparison": "gte", "expected": 130 },
    { "name": "spaces_with_app_aichat", "comparison": "eq", "expected": 7 },
    { "name": "version_skew_api_image", "comparison": "ne" },
    { "name": "target_us-dev-1_exists", "comparison": "gte", "expected": 1 },
    { "name": "target_us-prod-1_exists", "comparison": "gte", "expected": 1 },
    { "name": "target_eu-prod-1_exists", "comparison": "gte", "expected": 1 }
  ]
}
EOF
}

JSON_OUTPUT=false

case "${1:-}" in
  --explain)
    show_explain
    exit 0
    ;;
  --explain-json)
    show_explain_json
    exit 0
    ;;
  --json)
    JSON_OUTPUT=true
    ;;
  "")
    ;;
  *)
    echo "Usage: $0 [--json|--explain|--explain-json]" >&2
    exit 2
    ;;
esac

source "${DEMO_DIR}/lib.sh"

command -v "${CUB}" >/dev/null 2>&1 || { echo "error: cub CLI not found." >&2; exit 1; }

PASS=0
FAIL=0
CHECKS=()

json_escape() {
  local s="$1"
  s=${s//\\/\\\\}
  s=${s//\"/\\\"}
  s=${s//$'\n'/\\n}
  printf '%s' "$s"
}

record_result() {
  local name="$1"
  local comparison="$2"
  local expected="$3"
  local actual="$4"
  local status="$5"
  local json

  json=$(printf '{"name":"%s","comparison":"%s","expected":"%s","actual":"%s","status":"%s"}' \
    "$(json_escape "$name")" \
    "$(json_escape "$comparison")" \
    "$(json_escape "$expected")" \
    "$(json_escape "$actual")" \
    "$(json_escape "$status")")
  CHECKS+=("$json")
}

count_lines() {
  local output
  output="$("$@" 2>/dev/null || true)"
  if [[ -z "$output" ]]; then
    echo "0"
  else
    printf '%s\n' "$output" | wc -l | tr -d ' '
  fi
}

check() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  local status="PASS"

  if [[ "$actual" == "$expected" ]]; then
    if [[ "${JSON_OUTPUT}" != "true" ]]; then
      echo "  PASS: $name ($actual)"
    fi
    PASS=$((PASS + 1))
  else
    if [[ "${JSON_OUTPUT}" != "true" ]]; then
      echo "  FAIL: $name (expected $expected, got $actual)"
    fi
    FAIL=$((FAIL + 1))
    status="FAIL"
  fi

  record_result "$name" "eq" "$expected" "$actual" "$status"
}

check_gte() {
  local name="$1"
  local expected="$2"
  local actual="$3"
  local status="PASS"

  if [[ "$actual" -ge "$expected" ]]; then
    if [[ "${JSON_OUTPUT}" != "true" ]]; then
      echo "  PASS: $name ($actual >= $expected)"
    fi
    PASS=$((PASS + 1))
  else
    if [[ "${JSON_OUTPUT}" != "true" ]]; then
      echo "  FAIL: $name (expected >= $expected, got $actual)"
    fi
    FAIL=$((FAIL + 1))
    status="FAIL"
  fi

  record_result "$name" "gte" "$expected" "$actual" "$status"
}

emit_json_summary() {
  local overall_status="pass"
  if [[ "$FAIL" -gt 0 ]]; then
    overall_status="fail"
  fi

  printf '{\n'
  printf '  "example_name": "promotion-demo-data",\n'
  printf '  "status": "%s",\n' "$overall_status"
  printf '  "passed": %d,\n' "$PASS"
  printf '  "failed": %d,\n' "$FAIL"
  printf '  "checks": [\n'

  local i
  for ((i=0; i<${#CHECKS[@]}; i++)); do
    printf '    %s' "${CHECKS[$i]}"
    if (( i + 1 < ${#CHECKS[@]} )); then
      printf ','
    fi
    printf '\n'
  done

  printf '  ]\n'
  printf '}\n'
}

if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo "=== ConfigHub Demo Verification ==="
  echo ""
fi

##################################
# Check 1: Space counts
##################################
if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo "Checking space counts..."
fi

# Expected: 1 worker space + 7 infra spaces + 42 app spaces = 50
total_spaces=$(count_lines "${CUB}" space list --where "Labels.ExampleName = '${EXAMPLE_NAME}'" --no-header)
check_gte "total_demo_spaces" "49" "$total_spaces"

# Platform owns: 1 worker + 7 infra + 7 platform app spaces = 15
infra_spaces=$(count_lines "${CUB}" space list --where "Labels.ExampleName = '${EXAMPLE_NAME}' AND Labels.AppOwner = 'Platform'" --no-header)
check_gte "platform_owned_spaces" "8" "$infra_spaces"

# Prod: 2 targets × 6 apps + 2 infra = 14
prod_spaces=$(count_lines "${CUB}" space list --where "Labels.ExampleName = '${EXAMPLE_NAME}' AND Labels.TargetRole = 'Prod'" --no-header)
check_gte "prod_spaces" "12" "$prod_spaces"

if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo ""
fi

##################################
# Check 2: Unit counts
##################################
if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo "Checking unit counts..."
fi

# Count units across all demo spaces
total_units=0
for target in "${TARGETS[@]}"; do
  for app in "${APPS[@]}"; do
    space="${target}-${app}"
    count=$(count_lines "${CUB}" unit list --space "$space" --no-header)
    total_units=$((total_units + count))
  done
done
check_gte "total_units_across_app_spaces" "130" "$total_units"

if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo ""
fi

##################################
# Check 3: Label presence
##################################
if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo "Checking label presence..."
fi

# Check that prod units have TargetRole label
us_prod_eshop_units=$(count_lines "${CUB}" unit list --space us-prod-1-eshop --no-header)
check_gte "units_in_us-prod-1-eshop" "5" "$us_prod_eshop_units"

# Check label query works
aichat_spaces=$(count_lines "${CUB}" space list --where "Labels.App = 'aichat'" --no-header)
check "spaces_with_app_aichat" "7" "$aichat_spaces"

if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo ""
fi

##################################
# Check 4: Version skew exists
##################################
if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo "Checking version skew..."
fi

us_prod_api_image=$($CUB function do get-image api --space us-prod-1-eshop --unit api --output-only 2>/dev/null || echo "unknown")
eu_prod_api_image=$($CUB function do get-image api --space eu-prod-1-eshop --unit api --output-only 2>/dev/null || echo "unknown")

version_skew_status="PASS"
if [[ "$us_prod_api_image" != "$eu_prod_api_image" ]] && [[ "$us_prod_api_image" != "unknown" ]]; then
  if [[ "${JSON_OUTPUT}" != "true" ]]; then
    echo "  PASS: Version skew exists (us-prod-1: $us_prod_api_image, eu-prod-1: $eu_prod_api_image)"
  fi
  PASS=$((PASS + 1))
else
  if [[ "${JSON_OUTPUT}" != "true" ]]; then
    echo "  FAIL: Version skew not found or images unknown"
  fi
  FAIL=$((FAIL + 1))
  version_skew_status="FAIL"
fi

record_result \
  "version_skew_api_image" \
  "ne" \
  "us-prod-1-eshop != eu-prod-1-eshop" \
  "us-prod-1-eshop=${us_prod_api_image}; eu-prod-1-eshop=${eu_prod_api_image}" \
  "$version_skew_status"

if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo ""
fi

##################################
# Check 5: Targets exist
##################################
if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo "Checking targets..."
fi

for target in us-dev-1 us-prod-1 eu-prod-1; do
  target_exists=$(count_lines "${CUB}" target list --space "$target" --no-header)
  if [[ "$target_exists" -ge 1 ]]; then
    if [[ "${JSON_OUTPUT}" != "true" ]]; then
      echo "  PASS: Target $target exists"
    fi
    PASS=$((PASS + 1))
    record_result "target_${target}_exists" "gte" "1" "$target_exists" "PASS"
  else
    if [[ "${JSON_OUTPUT}" != "true" ]]; then
      echo "  FAIL: Target $target not found"
    fi
    FAIL=$((FAIL + 1))
    record_result "target_${target}_exists" "gte" "1" "$target_exists" "FAIL"
  fi
done

if [[ "${JSON_OUTPUT}" != "true" ]]; then
  echo ""
fi

##################################
# Summary
##################################
if [[ "${JSON_OUTPUT}" == "true" ]]; then
  emit_json_summary
else
  echo "=== Verification Complete ==="
  echo ""
  echo "  Passed: $PASS"
  echo "  Failed: $FAIL"
  echo ""
fi

if [[ "$FAIL" -gt 0 ]]; then
  if [[ "${JSON_OUTPUT}" != "true" ]]; then
    echo "Some checks failed. Run promotion-demo-data/setup.sh to create demo data."
  fi
  exit 1
else
  if [[ "${JSON_OUTPUT}" != "true" ]]; then
    echo "All checks passed."
  fi
  exit 0
fi
