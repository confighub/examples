#!/usr/bin/env bash
# Verify springboot-platform-app-centric
#
# Checks:
# 1. Local files in this wrapper exist
# 2. Delegates to parent example for fixture verification

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="${SCRIPT_DIR}/../springboot-platform-app"

# Check local wrapper files
local_files=(
  "${SCRIPT_DIR}/README.md"
  "${SCRIPT_DIR}/AI_START_HERE.md"
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
  if [[ ! -f "${file}" ]]; then
    echo "missing required file: ${file}" >&2
    exit 1
  fi
done

# Verify deployment-map.json structure
command -v jq >/dev/null 2>&1 || {
  echo "error: jq not found" >&2
  exit 1
}

jq -e '.app.name == "inventory-api"' "${SCRIPT_DIR}/deployment-map.json" >/dev/null
jq -e '.deployments | length == 3' "${SCRIPT_DIR}/deployment-map.json" >/dev/null
jq -e '.target_modes | keys | length == 3' "${SCRIPT_DIR}/deployment-map.json" >/dev/null
jq -e '.mutation_outcomes | length == 3' "${SCRIPT_DIR}/deployment-map.json" >/dev/null

echo "ok: springboot-platform-app-centric wrapper files are consistent"

# Delegate to parent verification
if [[ ! -f "${PARENT_DIR}/verify.sh" ]]; then
  echo "error: Parent verify.sh not found at ${PARENT_DIR}/verify.sh" >&2
  exit 1
fi

echo ""
echo "Delegating to parent example verification..."
exec "${PARENT_DIR}/verify.sh"
