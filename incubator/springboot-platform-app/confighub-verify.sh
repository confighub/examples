#!/usr/bin/env bash
# Verify that ConfigHub-only objects exist and are inspectable.
#
# Usage:
#   ./confighub-verify.sh

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app"
ENVS=(dev stage prod)
errors=0

echo "=== Verifying ConfigHub-only objects for springboot-platform-app ==="
echo ""

# Check spaces exist
space_count=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json | jq 'length')
if [[ "${space_count}" -ne 3 ]]; then
  echo "FAIL: expected 3 spaces, found ${space_count}" >&2
  errors=$((errors + 1))
else
  echo "ok: found ${space_count} spaces with ExampleName=${EXAMPLE_LABEL}"
fi

# Check each space has the inventory-api unit
for env in "${ENVS[@]}"; do
  space="inventory-api-${env}"
  unit_json=$(${CUB} unit get --space "${space}" --json inventory-api 2>&1) || {
    echo "FAIL: unit inventory-api not found in space ${space}" >&2
    errors=$((errors + 1))
    continue
  }

  # Check unit has data content
  data_len=$(echo "${unit_json}" | jq '.Unit.Data | length')
  if [[ "${data_len}" -lt 1 ]]; then
    echo "FAIL: unit ${space}/inventory-api has no data" >&2
    errors=$((errors + 1))
  else
    echo "ok: ${space}/inventory-api has data (${data_len} bytes)"
  fi
done

echo ""
if [[ "${errors}" -gt 0 ]]; then
  echo "FAIL: ${errors} error(s) found"
  exit 1
else
  echo "ok: springboot-platform-app ConfigHub-only objects are consistent"
fi
