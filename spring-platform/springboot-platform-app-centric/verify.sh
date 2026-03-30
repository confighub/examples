#!/usr/bin/env bash
# Verify springboot-platform-app-centric fixtures
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

# Check local ADT files
local_files=(
  "${SCRIPT_DIR}/README.md"
  "${SCRIPT_DIR}/AI_START_HERE.md"
  "${SCRIPT_DIR}/prompts.md"
  "${SCRIPT_DIR}/contracts.md"
  "${SCRIPT_DIR}/deployment-map.json"
  "${SCRIPT_DIR}/setup.sh"
  "${SCRIPT_DIR}/demo.sh"
  "${SCRIPT_DIR}/verify.sh"
  "${SCRIPT_DIR}/cleanup.sh"
  "${SCRIPT_DIR}/flows/apply-here.md"
  "${SCRIPT_DIR}/flows/lift-upstream.md"
  "${SCRIPT_DIR}/flows/block-escalate.md"
)

for file in "${local_files[@]}"; do
  check_file "${file}"
done

# Check shared files this example depends on
shared_files=(
  "${SHARED_DIR}/confighub/inventory-api-dev.yaml"
  "${SHARED_DIR}/confighub/inventory-api-stage.yaml"
  "${SHARED_DIR}/confighub/inventory-api-prod.yaml"
  "${SHARED_DIR}/field-routes.yaml"
)

for file in "${shared_files[@]}"; do
  check_file "${file}"
done

# Check jq is available
command -v jq >/dev/null 2>&1 || {
  echo "error: jq not found" >&2
  exit 1
}

# Verify deployment-map.json structure
jq -e '.app.name == "inventory-api"' "${SCRIPT_DIR}/deployment-map.json" >/dev/null || {
  echo "error: deployment-map.json missing app.name" >&2
  errors=$((errors + 1))
}

jq -e '.deployments | length == 3' "${SCRIPT_DIR}/deployment-map.json" >/dev/null || {
  echo "error: deployment-map.json should have 3 deployments" >&2
  errors=$((errors + 1))
}

jq -e '.target_modes | keys | length == 3' "${SCRIPT_DIR}/deployment-map.json" >/dev/null || {
  echo "error: deployment-map.json should have 3 target modes" >&2
  errors=$((errors + 1))
}

jq -e '.mutation_outcomes | length == 3' "${SCRIPT_DIR}/deployment-map.json" >/dev/null || {
  echo "error: deployment-map.json should have 3 mutation outcomes" >&2
  errors=$((errors + 1))
}

# Verify setup.sh --explain-json output
"${SCRIPT_DIR}/setup.sh" --explain-json | jq -e '.example_name == "springboot-platform-app-centric"' >/dev/null || {
  echo "error: setup.sh --explain-json has wrong example_name" >&2
  errors=$((errors + 1))
}

"${SCRIPT_DIR}/setup.sh" --explain-json | jq -e '.proof_type == "adt-view"' >/dev/null || {
  echo "error: setup.sh --explain-json has wrong proof_type" >&2
  errors=$((errors + 1))
}

# Check scripts are executable
for script in setup.sh verify.sh cleanup.sh demo.sh; do
  if [[ ! -x "${SCRIPT_DIR}/${script}" ]]; then
    echo "warning: ${script} is not executable" >&2
  fi
done

if [[ ${errors} -gt 0 ]]; then
  echo "fail: ${errors} error(s) found" >&2
  exit 1
fi

echo "ok: springboot-platform-app-centric fixtures are consistent"
