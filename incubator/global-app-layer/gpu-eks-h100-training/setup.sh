#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq

if state_exists; then
  echo "State already exists in ${STATE_FILE}. Run ./cleanup.sh first or remove .state if you know it is stale." >&2
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

echo "==> Creating base unit"
_mapfile base_unit_labels < <(label_args base)
create_unit_from_file "$(base_space)" "$(unit_name base)" "${BASE_MANIFEST}" "${base_unit_labels[@]}"

echo "==> Creating platform clone"
_mapfile platform_unit_labels < <(label_args platform --label "Platform=${PLATFORM_VALUE}")
create_clone_unit "$(platform_space)" "$(unit_name platform)" "$(base_space)" "$(unit_name base)" "${platform_unit_labels[@]}"
apply_platform_mutations

echo "==> Creating accelerator clone"
_mapfile accelerator_unit_labels < <(label_args accelerator --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}")
create_clone_unit "$(accelerator_space)" "$(unit_name accelerator)" "$(platform_space)" "$(unit_name platform)" "${accelerator_unit_labels[@]}"
apply_accelerator_mutations

echo "==> Creating OS clone"
_mapfile os_unit_labels < <(label_args os --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}")
create_clone_unit "$(os_space)" "$(unit_name os)" "$(accelerator_space)" "$(unit_name accelerator)" "${os_unit_labels[@]}"
apply_os_mutations

echo "==> Creating recipe clone"
_mapfile recipe_unit_labels < <(label_args recipe --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}")
create_clone_unit "$(recipe_space)" "$(unit_name recipe)" "$(os_space)" "$(unit_name os)" "${recipe_unit_labels[@]}"
apply_recipe_mutations

echo "==> Creating deployment clone"
_mapfile deploy_unit_labels < <(label_args deployment --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}")
create_clone_unit "$(deploy_space)" "$(unit_name deployment)" "$(recipe_space)" "$(unit_name recipe)" "${deploy_unit_labels[@]}"
apply_deploy_mutations

if [[ -n "${TARGET_REF}" ]]; then
  echo "==> Setting target on deployment clone"
  set_target_for_deploy_unit "${TARGET_REF}"
fi

echo "==> Rendering explicit GPU recipe manifest"
refresh_recipe_manifest_unit "${TARGET_REF}"

show_summary "${TARGET_REF}"
