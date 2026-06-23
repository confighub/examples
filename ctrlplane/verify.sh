#!/usr/bin/env bash
#
# Verify the ctrlplane-on-confighub mapper produces a valid, stable plan.
# READ-ONLY: only runs the mapper's read-only modes.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
fail=0

check() {
  local desc="$1"; shift
  if "$@" >/dev/null 2>&1; then
    echo "  PASS: ${desc}"
  else
    echo "  FAIL: ${desc}" >&2
    fail=1
  fi
}

echo "==> ctrlplane-on-confighub verify"

# 1. explain runs
check "setup.sh --explain runs" bash "${here}/setup.sh" --explain

# 2. explain-json is valid JSON
check "setup.sh --explain-json emits valid JSON" \
  bash -c "bash '${here}/setup.sh' --explain-json | jq -e ."

# 3. stable fields present and correct
json="$(bash "${here}/setup.sh" --explain-json)"
check "mutates is false" \
  bash -c "echo '${json}' | jq -e '.mutates == false'"
check "three spaces proposed (base + 2 envs)" \
  bash -c "echo '${json}' | jq -e '.spaces | length == 3'"
check "base space present" \
  bash -c "echo '${json}' | jq -e '[.spaces[] | select(endswith(\"-base\"))] | length == 1'"
check "one base unit (upstream)" \
  bash -c "echo '${json}' | jq -e '[.plan.units[] | select(.role == \"base\")] | length == 1'"
check "two variant units linked to upstream" \
  bash -c "echo '${json}' | jq -e '[.plan.units[] | select(.role == \"variant\" and .upstream != null)] | length == 2'"
check "three targets proposed" \
  bash -c "echo '${json}' | jq -e '.targets | length == 3'"
check "delivery strategy is confighub-oci-argo" \
  bash -c "echo '${json}' | jq -e '.delivery_strategy == \"confighub-oci-argo\"'"
check "mapping_notes present" \
  bash -c "echo '${json}' | jq -e '.mapping_notes | length >= 1'"
check "generated commands use --upstream-unit" \
  bash -c "bash '${here}/setup.sh' --cub-commands | grep -q -- '--upstream-unit'"

# 4. cub-commands path runs and is shell-parseable
check "setup.sh --cub-commands runs" bash "${here}/setup.sh" --cub-commands
check "generated cub-commands are valid shell" \
  bash -c "bash '${here}/setup.sh' --cub-commands | bash -n /dev/stdin"

if [[ "${fail}" -eq 0 ]]; then
  echo "All checks passed."
else
  echo "Some checks failed." >&2
  exit 1
fi
