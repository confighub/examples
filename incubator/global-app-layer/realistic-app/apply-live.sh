#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
begin_log_capture apply-live
load_state

PREFLIGHT_SCRIPT="${SCRIPT_DIR}/../preflight-live.sh"

target_ref="${1:-${TARGET_REF:-}}"
if [[ -z "${target_ref}" ]]; then
  echo "Usage: $0 [<space/target-or-target-slug>]" >&2
  echo "Run ./set-target.sh first, or pass a target ref here." >&2
  exit 1
fi

ensure_apply_ready_target() {
  local target="$1"
  local preflight_json

  if [[ ! -f "${PREFLIGHT_SCRIPT}" ]]; then
    echo "Missing required helper: ${PREFLIGHT_SCRIPT}" >&2
    exit 1
  fi

  echo "==> Preflighting live target ${target}"
  preflight_json="$(bash "${PREFLIGHT_SCRIPT}" "${target}" --json)"

  if ! jq -e '.applyReady == true' >/dev/null <<<"${preflight_json}"; then
    echo "${preflight_json}" | jq '{targetRef, applyReady, providerType, deliveryMode, worker: .bridgeWorker, reasons}'
    echo "Target is not ready for live apply. Fix the worker/target and rerun." >&2
    exit 1
  fi

  echo "${preflight_json}" | jq '{targetRef, applyReady, providerType, deliveryMode, worker: .bridgeWorker}'
}

upgrade_deploy_units_from_upstream() {
  local component unit
  echo "==> Refreshing deployment units from upstream recipe revisions"
  for component in "${COMPONENTS[@]}"; do
    unit="$(unit_name "${component}" deployment)"
    cub unit update --quiet --space "$(deploy_space)" "${unit}" --upgrade >/dev/null
    echo "  refreshed ${unit} -> head revision $(get_unit_field "$(deploy_space)" "${unit}" HeadRevisionNum)"
  done
}

ensure_apply_ready_target "${target_ref}"
assert_supported_live_target "${target_ref}"

if [[ "${target_ref}" != "${TARGET_REF:-}" ]]; then
  echo "==> Updating target binding to ${target_ref}"
  set_target_for_deploy_units "${target_ref}"
  save_state "${PREFIX}" "${target_ref}"
  TARGET_REF="${target_ref}"
else
  ensure_namespace_unit "${target_ref}"
fi

upgrade_deploy_units_from_upstream
refresh_recipe_manifest_unit "${target_ref}"

wait_for_apply_result() {
  local unit="$1"
  local timeout_seconds="${2:-180}"
  local start now status action_result
  start="$(date +%s)"

  while true; do
    status="$(cub unit get --space "$(deploy_space)" --json "${unit}" | jq -r '.UnitStatus.Status')"
    action_result="$(cub unit get --space "$(deploy_space)" --json "${unit}" | jq -r '.UnitStatus.ActionResult')"

    case "${action_result}" in
      ApplyCompleted)
        echo "  ${unit}: ${status} / ${action_result}"
        return 0
        ;;
      ApplyFailed)
        echo "  ${unit}: ${status} / ${action_result}" >&2
        cub unit-event list "${unit}" --space "$(deploy_space)" | tail -10 >&2 || true
        return 1
        ;;
    esac

    now="$(date +%s)"
    if (( now - start >= timeout_seconds )); then
      echo "Timed out waiting for ${unit}. Current status=${status}, actionResult=${action_result}" >&2
      cub unit-event list "${unit}" --space "$(deploy_space)" | tail -10 >&2 || true
      return 1
    fi

    echo "  ${unit}: ${status} / ${action_result} ... waiting"
    sleep 5
  done
}

apply_unit() {
  local unit="$1"
  echo "==> Approving ${unit}"
  cub unit approve --space "$(deploy_space)" "${unit}"
  echo "==> Applying ${unit}"
  cub unit apply --space "$(deploy_space)" "${unit}"
}

echo "==> Applying deployment bootstrap unit first"
namespace_unit="$(namespace_unit_name)"
apply_unit "${namespace_unit}"
wait_for_apply_result "${namespace_unit}" 120

echo "==> Applying realistic-app deployment units"
for component in "${COMPONENTS[@]}"; do
  apply_unit "$(unit_name "${component}" deployment)"
done

for component in "${COMPONENTS[@]}"; do
  wait_for_apply_result "$(unit_name "${component}" deployment)" 300
done

echo "==> Final deployment status"
cub unit list --space "$(deploy_space)" --quiet --json | jq '.[] | {slug: .Unit.Slug, status: .UnitStatus.Status, actionResult: .UnitStatus.ActionResult}'

echo "Live apply completed successfully for $(state_prefix)."
