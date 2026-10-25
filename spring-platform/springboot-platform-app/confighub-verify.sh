#!/usr/bin/env bash
# Verify that ConfigHub objects exist and are inspectable.
#
# Usage:
#   ./confighub-verify.sh

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app"
ENVS=(dev stage prod)
errors=0

echo "=== Verifying ConfigHub objects for springboot-platform-app ==="
echo ""

# Check spaces exist
space_count=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json | jq 'length')
expected=3

if [[ "${space_count}" -lt "${expected}" ]]; then
  echo "FAIL: expected at least ${expected} spaces, found ${space_count}" >&2
  errors=$((errors + 1))
else
  echo "ok: found ${space_count} spaces with ExampleName=${EXAMPLE_LABEL}"
fi

# Check each env space has the inventory-api unit
for env in "${ENVS[@]}"; do
  space="inventory-api-${env}"
  unit_json=$(${CUB} unit get --space "${space}" --json inventory-api 2>&1) || {
    echo "FAIL: unit inventory-api not found in space ${space}" >&2
    errors=$((errors + 1))
    continue
  }

  # Check unit has data content. The configuration is not part of the Unit any
  # more; DataSize is the byte count the server reports for it.
  data_len=$(echo "${unit_json}" | jq '.Unit.DataSize // 0')
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
  echo "ok: springboot-platform-app ConfigHub objects are consistent"
fi
