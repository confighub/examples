#!/usr/bin/env bash
# Verify springboot-platform-platform-centric fixtures
#
# Checks that all required files exist and are consistent.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="${SCRIPT_DIR}/../shared"

errors=0

check_file() {
  if [[ ! -f "$1" ]]; then
    echo "error: missing $1" >&2
    errors=$((errors + 1))
  fi
}

check_dir() {
  if [[ ! -d "$1" ]]; then
    echo "error: missing directory $1" >&2
    errors=$((errors + 1))
  fi
}

# Check required files
check_file "${SCRIPT_DIR}/README.md"
check_file "${SCRIPT_DIR}/platform.yaml"
check_file "${SCRIPT_DIR}/platform-map.json"
check_file "${SCRIPT_DIR}/setup.sh"
check_file "${SCRIPT_DIR}/cleanup.sh"
check_file "${SCRIPT_DIR}/platform.sh"

# Check catalog-api fixtures
check_file "${SCRIPT_DIR}/apps/catalog-api/dev.yaml"
check_file "${SCRIPT_DIR}/apps/catalog-api/prod.yaml"

# Check shared resources exist
check_dir "${SHARED_DIR}"
check_file "${SHARED_DIR}/confighub/inventory-api-dev.yaml"
check_file "${SHARED_DIR}/confighub/inventory-api-stage.yaml"
check_file "${SHARED_DIR}/confighub/inventory-api-prod.yaml"

# Check platform-map.json is valid JSON
if ! jq empty "${SCRIPT_DIR}/platform-map.json" 2>/dev/null; then
  echo "error: platform-map.json is not valid JSON" >&2
  errors=$((errors + 1))
fi

# Check platform-map.json has expected structure
if ! jq -e '.platform.name == "springboot-platform"' "${SCRIPT_DIR}/platform-map.json" >/dev/null 2>&1; then
  echo "error: platform-map.json missing platform.name" >&2
  errors=$((errors + 1))
fi

if ! jq -e '.apps | length == 2' "${SCRIPT_DIR}/platform-map.json" >/dev/null 2>&1; then
  echo "error: platform-map.json should have 2 apps" >&2
  errors=$((errors + 1))
fi

# Verify target modes match actual implementation
if ! jq -e '.target_modes.noop.implemented == true' "${SCRIPT_DIR}/platform-map.json" >/dev/null 2>&1; then
  echo "error: platform-map.json should mark noop as implemented" >&2
  errors=$((errors + 1))
fi

if ! jq -e '.target_modes.real.implemented == false' "${SCRIPT_DIR}/platform-map.json" >/dev/null 2>&1; then
  echo "error: platform-map.json should mark real as not implemented" >&2
  errors=$((errors + 1))
fi

# Check scripts are executable
for script in setup.sh cleanup.sh platform.sh; do
  if [[ ! -x "${SCRIPT_DIR}/${script}" ]]; then
    echo "warning: ${script} is not executable" >&2
  fi
done

if [[ ${errors} -gt 0 ]]; then
  echo "fail: ${errors} error(s) found" >&2
  exit 1
fi

echo "ok: springboot-platform-platform-centric fixtures are consistent"
