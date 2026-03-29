#!/usr/bin/env bash
# Verify that the platform-write-api example is structurally consistent.
#
# Checks:
# - Required scripts exist
# - Dependency on springboot-platform-app fixtures is satisfied
# - All scripts have --explain mode

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPRING_DIR="${SCRIPT_DIR}/../springboot-platform-app"
ok=true

check_file() {
  if [[ ! -f "$1" ]]; then
    echo "FAIL: missing $1" >&2
    ok=false
  fi
}

check_executable() {
  if [[ ! -x "$1" ]]; then
    echo "FAIL: not executable $1" >&2
    ok=false
  fi
}

# Own scripts
for script in setup.sh compare.sh field-routes.sh refresh-preview.sh mutate.sh cleanup.sh verify.sh; do
  check_file "${SCRIPT_DIR}/${script}"
  check_executable "${SCRIPT_DIR}/${script}"
done

# Dependency: springboot-platform-app fixtures
for env in dev stage prod; do
  check_file "${SPRING_DIR}/confighub/inventory-api-${env}.yaml"
done
check_file "${SPRING_DIR}/operational/field-routes.yaml"

# Dependency: springboot-platform-app scripts we delegate to
for script in confighub-compare.sh confighub-field-routes.sh confighub-refresh-preview.sh; do
  check_file "${SPRING_DIR}/${script}"
  check_executable "${SPRING_DIR}/${script}"
done

command -v jq >/dev/null 2>&1 || {
  echo "FAIL: jq not found" >&2
  ok=false
}

# Check explain modes work
for script in setup.sh mutate.sh; do
  if ! "${SCRIPT_DIR}/${script}" --explain > /dev/null 2>&1; then
    echo "FAIL: ${script} --explain failed" >&2
    ok=false
  fi
done

check_file "${SCRIPT_DIR}/README.md"
check_file "${SCRIPT_DIR}/AI_START_HERE.md"
check_file "${SCRIPT_DIR}/prompts.md"
check_file "${SCRIPT_DIR}/contracts.md"

if ! "${SCRIPT_DIR}/setup.sh" --explain-json | jq -e '.example_name == "platform-write-api"' >/dev/null 2>&1; then
  echo "FAIL: setup.sh --explain-json contract invalid" >&2
  ok=false
fi

if ! "${SCRIPT_DIR}/compare.sh" --json | jq -e 'has("feature.inventory.reservationMode")' >/dev/null 2>&1; then
  echo "FAIL: compare.sh --json contract invalid" >&2
  ok=false
fi

if ! "${SCRIPT_DIR}/field-routes.sh" prod --json | jq -e 'type == "array" and length >= 3' >/dev/null 2>&1; then
  echo "FAIL: field-routes.sh --json contract invalid" >&2
  ok=false
fi

if ! "${SCRIPT_DIR}/refresh-preview.sh" prod --json | jq -e 'type == "array" and length >= 1' >/dev/null 2>&1; then
  echo "FAIL: refresh-preview.sh --json contract invalid" >&2
  ok=false
fi

if ! "${SCRIPT_DIR}/lift-upstream.sh" --json | jq -e '.routing.route == "lift-upstream"' >/dev/null 2>&1; then
  echo "FAIL: lift-upstream.sh --json contract invalid" >&2
  ok=false
fi

if ! "${SCRIPT_DIR}/block-escalate.sh" --json | jq -e '.routing.blocked == true' >/dev/null 2>&1; then
  echo "FAIL: block-escalate.sh --json contract invalid" >&2
  ok=false
fi

if [[ "$ok" == "true" ]]; then
  echo "ok: platform-write-api example is consistent"
else
  echo "FAIL: platform-write-api has issues" >&2
  exit 1
fi
