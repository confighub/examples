#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_python
load_state

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <space/target-or-target-slug>" >&2
  exit 1
fi

target_ref="$1"

cub unit set-target "${target_ref}" --space "$(deploy_space)" --unit "${DEPLOY_UNIT}"
save_state "${PREFIX}" "${target_ref}"
load_state
refresh_recipe_manifest_unit "${TARGET_REF}"

echo "Updated deployment target for $(deploy_space)/${DEPLOY_UNIT}: ${TARGET_REF}"
echo "Bundle hint: $(bundle_hint_from_target_ref "${TARGET_REF}")"
