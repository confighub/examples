#!/usr/bin/env bash
# Verify that ConfigHub objects exist and are inspectable.
#
# Usage:
#   ./confighub-verify.sh               # Verify spaces and units
#   ./confighub-verify.sh --targets     # Also verify targets and apply status

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app"
ENVS=(dev stage prod)
CHECK_TARGETS=false
errors=0

if [[ "${1:-}" == "--targets" ]]; then
  CHECK_TARGETS=true
fi

echo "=== Verifying ConfigHub objects for springboot-platform-app ==="
echo ""

# Check spaces exist
space_count=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json | jq 'length')
if [[ "${CHECK_TARGETS}" == "true" ]]; then
  expected=4
else
  expected=3
fi
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

  # Check unit has data content
  data_len=$(echo "${unit_json}" | jq '.Unit.Data | length')
  if [[ "${data_len}" -lt 1 ]]; then
    echo "FAIL: unit ${space}/inventory-api has no data" >&2
    errors=$((errors + 1))
  else
    echo "ok: ${space}/inventory-api has data (${data_len} bytes)"
  fi

  # If checking targets, verify unit status
  if [[ "${CHECK_TARGETS}" == "true" ]]; then
    status=$(echo "${unit_json}" | jq -r '.UnitStatus.Status')
    sync=$(echo "${unit_json}" | jq -r '.UnitStatus.SyncStatus')
    if [[ "${status}" != "Ready" ]]; then
      echo "FAIL: ${space}/inventory-api status is ${status}, expected Ready" >&2
      errors=$((errors + 1))
    else
      echo "ok: ${space}/inventory-api status=${status} sync=${sync}"
    fi
  fi
done

# If checking targets, verify infra space and targets
if [[ "${CHECK_TARGETS}" == "true" ]]; then
  infra_json=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json \
    | jq '[.[] | select(.Space.Slug == "inventory-api-infra")]')
  infra_count=$(echo "${infra_json}" | jq 'length')
  if [[ "${infra_count}" -lt 1 ]]; then
    echo "FAIL: infra space inventory-api-infra not found" >&2
    errors=$((errors + 1))
  else
    echo "ok: infra space inventory-api-infra exists"
  fi

  for env in "${ENVS[@]}"; do
    space="inventory-api-${env}"
    target_count=$(${CUB} target list --space "${space}" --json 2>&1 | jq 'length')
    if [[ "${target_count}" -lt 1 ]]; then
      echo "FAIL: no target in ${space}" >&2
      errors=$((errors + 1))
    else
      echo "ok: ${space} has ${target_count} target(s)"
    fi
  done
fi

echo ""
if [[ "${errors}" -gt 0 ]]; then
  echo "FAIL: ${errors} error(s) found"
  exit 1
else
  if [[ "${CHECK_TARGETS}" == "true" ]]; then
    echo "ok: springboot-platform-app ConfigHub objects with targets are consistent"
  else
    echo "ok: springboot-platform-app ConfigHub objects are consistent"
  fi
fi
