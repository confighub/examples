#!/bin/bash
# Verification wrapper for promotion-demo-data
#
# This script lives in incubator/ because the promotion-demo-data example
# is a stable example that shouldn't have incubator-style scripts added to it.
#
# Usage:
#   ./verify.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEMO_DIR="${SCRIPT_DIR}/../../promotion-demo-data"

source "${DEMO_DIR}/lib.sh"

echo "=== ConfigHub Demo Verification ==="
echo ""

PASS=0
FAIL=0

check() {
  local name="$1"
  local expected="$2"
  local actual="$3"

  if [[ "$actual" == "$expected" ]]; then
    echo "  PASS: $name ($actual)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name (expected $expected, got $actual)"
    FAIL=$((FAIL + 1))
  fi
}

check_gte() {
  local name="$1"
  local expected="$2"
  local actual="$3"

  if [[ "$actual" -ge "$expected" ]]; then
    echo "  PASS: $name ($actual >= $expected)"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: $name (expected >= $expected, got $actual)"
    FAIL=$((FAIL + 1))
  fi
}

##################################
# Check 1: Space counts
##################################
echo "Checking space counts..."

# Expected: 1 worker space + 7 infra spaces + 42 app spaces = 50
total_spaces=$($CUB space list --where "Labels.ExampleName = '${EXAMPLE_NAME}'" --no-header 2>/dev/null | wc -l | tr -d ' ')
check_gte "Total demo spaces" "49" "$total_spaces"

# Platform owns: 1 worker + 7 infra + 7 platform app spaces = 15
infra_spaces=$($CUB space list --where "Labels.ExampleName = '${EXAMPLE_NAME}' AND Labels.AppOwner = 'Platform'" --no-header 2>/dev/null | wc -l | tr -d ' ')
check_gte "Platform-owned spaces" "8" "$infra_spaces"

# Prod: 2 targets × 6 apps + 2 infra = 14
prod_spaces=$($CUB space list --where "Labels.ExampleName = '${EXAMPLE_NAME}' AND Labels.TargetRole = 'Prod'" --no-header 2>/dev/null | wc -l | tr -d ' ')
check_gte "Prod spaces" "12" "$prod_spaces"

echo ""

##################################
# Check 2: Unit counts
##################################
echo "Checking unit counts..."

# Count units across all demo spaces
total_units=0
for target in "${TARGETS[@]}"; do
  for app in "${APPS[@]}"; do
    space="${target}-${app}"
    count=$($CUB unit list --space "$space" --no-header 2>/dev/null | wc -l | tr -d ' ')
    total_units=$((total_units + count))
  done
done
check_gte "Total units across app spaces" "130" "$total_units"

echo ""

##################################
# Check 3: Label presence
##################################
echo "Checking label presence..."

# Check that prod units have TargetRole label
us_prod_eshop_units=$($CUB unit list --space us-prod-1-eshop --no-header 2>/dev/null | wc -l | tr -d ' ')
check_gte "Units in us-prod-1-eshop" "5" "$us_prod_eshop_units"

# Check label query works
aichat_spaces=$($CUB space list --where "Labels.App = 'aichat'" --no-header 2>/dev/null | wc -l | tr -d ' ')
check "Spaces with App=aichat" "7" "$aichat_spaces"

echo ""

##################################
# Check 4: Version skew exists
##################################
echo "Checking version skew..."

us_prod_api_image=$($CUB function do get-image api --space us-prod-1-eshop --unit api --output-only 2>/dev/null || echo "unknown")
eu_prod_api_image=$($CUB function do get-image api --space eu-prod-1-eshop --unit api --output-only 2>/dev/null || echo "unknown")

if [[ "$us_prod_api_image" != "$eu_prod_api_image" ]] && [[ "$us_prod_api_image" != "unknown" ]]; then
  echo "  PASS: Version skew exists (us-prod-1: $us_prod_api_image, eu-prod-1: $eu_prod_api_image)"
  PASS=$((PASS + 1))
else
  echo "  FAIL: Version skew not found or images unknown"
  FAIL=$((FAIL + 1))
fi

echo ""

##################################
# Check 5: Targets exist
##################################
echo "Checking targets..."

for target in us-dev-1 us-prod-1 eu-prod-1; do
  target_exists=$($CUB target list --space "$target" --no-header 2>/dev/null | wc -l | tr -d ' ')
  if [[ "$target_exists" -ge 1 ]]; then
    echo "  PASS: Target $target exists"
    PASS=$((PASS + 1))
  else
    echo "  FAIL: Target $target not found"
    FAIL=$((FAIL + 1))
  fi
done

echo ""

##################################
# Summary
##################################
echo "=== Verification Complete ==="
echo ""
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo ""

if [[ "$FAIL" -gt 0 ]]; then
  echo "Some checks failed. Run promotion-demo-data/setup.sh to create demo data."
  exit 1
else
  echo "All checks passed."
  exit 0
fi
