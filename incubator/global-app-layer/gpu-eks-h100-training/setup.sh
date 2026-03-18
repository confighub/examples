#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq

if state_exists; then
  echo "State already exists in ${STATE_FILE}. The .state directory is local run state from an earlier setup. Run ./cleanup.sh first, or remove .state if you know this old state is no longer needed." >&2
  exit 1
fi

prefix="${1:-}"
target_ref="${2:-}"
if [[ -z "${prefix}" ]]; then
  prefix="$(cub space new-prefix)"
fi

save_state "${prefix}" "${target_ref}"
load_state

_mapfile base_space_labels < <(space_label_args base)
_mapfile platform_space_labels < <(space_label_args platform --label "Platform=${PLATFORM_VALUE}")
_mapfile accelerator_space_labels < <(space_label_args accelerator --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}")
_mapfile os_space_labels < <(space_label_args os --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}")
_mapfile recipe_space_labels < <(space_label_args recipe --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}")
_mapfile deploy_space_labels < <(space_label_args deployment --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}")

echo "==> Creating spaces"
create_space_if_missing "$(base_space)" "${base_space_labels[@]}"
create_space_if_missing "$(platform_space)" "${platform_space_labels[@]}"
create_space_if_missing "$(accelerator_space)" "${accelerator_space_labels[@]}"
create_space_if_missing "$(os_space)" "${os_space_labels[@]}"
create_space_if_missing "$(recipe_space)" "${recipe_space_labels[@]}"
create_space_if_missing "$(deploy_space)" "${deploy_space_labels[@]}"

for component in "${COMPONENTS[@]}"; do
  echo "==> Creating base unit for ${component}"
  _mapfile base_unit_labels < <(label_args base "${component}")
  create_unit_from_file "$(base_space)" "$(unit_name "${component}" base)" "$(source_yaml_for "${component}")" "${base_unit_labels[@]}"

  echo "==> Creating platform clone for ${component}"
  _mapfile platform_unit_labels < <(label_args platform "${component}" --label "Platform=${PLATFORM_VALUE}")
  create_clone_unit "$(platform_space)" "$(unit_name "${component}" platform)" "$(base_space)" "$(unit_name "${component}" base)" "${platform_unit_labels[@]}"
  apply_platform_mutations "${component}"

  echo "==> Creating accelerator clone for ${component}"
  _mapfile accelerator_unit_labels < <(label_args accelerator "${component}" --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}")
  create_clone_unit "$(accelerator_space)" "$(unit_name "${component}" accelerator)" "$(platform_space)" "$(unit_name "${component}" platform)" "${accelerator_unit_labels[@]}"
  apply_accelerator_mutations "${component}"

  echo "==> Creating OS clone for ${component}"
  _mapfile os_unit_labels < <(label_args os "${component}" --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}")
  create_clone_unit "$(os_space)" "$(unit_name "${component}" os)" "$(accelerator_space)" "$(unit_name "${component}" accelerator)" "${os_unit_labels[@]}"
  apply_os_mutations "${component}"

  echo "==> Creating recipe clone for ${component}"
  _mapfile recipe_unit_labels < <(label_args recipe "${component}" --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}")
  create_clone_unit "$(recipe_space)" "$(unit_name "${component}" recipe)" "$(os_space)" "$(unit_name "${component}" os)" "${recipe_unit_labels[@]}"
  apply_recipe_mutations "${component}"

  echo "==> Creating deployment clone for ${component}"
  _mapfile deploy_unit_labels < <(label_args deployment "${component}" --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}")
  create_clone_unit "$(deploy_space)" "$(unit_name "${component}" deployment)" "$(recipe_space)" "$(unit_name "${component}" recipe)" "${deploy_unit_labels[@]}"
  apply_deploy_mutations "${component}"
done

if [[ -n "${TARGET_REF}" ]]; then
  echo "==> Setting target on deployment clones"
  set_target_for_deploy_units "${TARGET_REF}"
fi

echo "==> Rendering explicit GPU recipe manifest"
refresh_recipe_manifest_unit "${TARGET_REF}"

show_summary "${TARGET_REF}"
