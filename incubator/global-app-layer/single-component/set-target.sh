#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
begin_log_capture set-target
load_state

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <space/target-or-target-slug> [additional-targets...]" >&2
  echo "" >&2
  supported_target_description >&2
  exit 1
fi

for target_ref in "$@"; do
  assert_supported_live_target "${target_ref}"
  echo "==> Setting target ${target_ref}"
  set_target_for_compatible_units "${target_ref}"
done

save_state "${PREFIX}" "${DIRECT_TARGET_REF:-}" "${FLUX_TARGET_REF:-}"
refresh_recipe_manifest_unit "${DIRECT_TARGET_REF:-}" "${FLUX_TARGET_REF:-}"

echo ""
echo "Target configuration:"
echo "- direct variant target: ${DIRECT_TARGET_REF:-<unset>}"
echo "- flux variant target: ${FLUX_TARGET_REF:-<unset>}"
echo ""
if [[ -n "${DIRECT_TARGET_REF:-}" ]]; then
  echo "- direct bundle hint: $(bundle_hint_from_target_ref "${DIRECT_TARGET_REF}")"
fi
if [[ -n "${FLUX_TARGET_REF:-}" ]]; then
  echo "- flux bundle hint: $(bundle_hint_from_target_ref "${FLUX_TARGET_REF}")"
fi

show_summary
