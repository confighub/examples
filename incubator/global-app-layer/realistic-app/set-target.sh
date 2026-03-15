#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
load_state

if [[ $# -ne 1 ]]; then
  echo "Usage: $0 <space/target-or-target-slug>" >&2
  exit 1
fi

target_ref="$1"

set_target_for_deploy_units "${target_ref}"
save_state "${PREFIX}" "${target_ref}"
TARGET_REF="${target_ref}"
refresh_recipe_manifest_unit "${target_ref}"

echo "Updated deployment target for $(deploy_space): backend + frontend + postgres => ${target_ref}"
echo "Bundle hint: $(bundle_hint_from_target_ref "${target_ref}")"
