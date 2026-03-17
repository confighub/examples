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
_mapfile region_space_labels < <(space_label_args region --label "Region=${REGION_VALUE}")
_mapfile role_space_labels < <(space_label_args role --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}")
_mapfile recipe_space_labels < <(space_label_args recipe --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}")
_mapfile deploy_space_labels < <(space_label_args deployment --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}")

echo "==> Creating spaces"
create_space_if_missing "$(base_space)" "${base_space_labels[@]}"
create_space_if_missing "$(region_space)" "${region_space_labels[@]}"
create_space_if_missing "$(role_space)" "${role_space_labels[@]}"
create_space_if_missing "$(recipe_space)" "${recipe_space_labels[@]}"
create_space_if_missing "$(deploy_space)" "${deploy_space_labels[@]}"

for component in "${COMPONENTS[@]}"; do
  echo "==> Creating base unit for ${component}"
  _mapfile base_unit_labels < <(label_args base "${component}")
  create_unit_from_file "$(base_space)" "$(unit_name "${component}" base)" "$(source_yaml_for "${component}")" "${base_unit_labels[@]}"

  echo "==> Creating region clone for ${component}"
  _mapfile region_unit_labels < <(label_args region "${component}" --label "Region=${REGION_VALUE}")
  create_clone_unit "$(region_space)" "$(unit_name "${component}" region)" "$(base_space)" "$(unit_name "${component}" base)" "${region_unit_labels[@]}"
  apply_region_mutations "${component}"

  echo "==> Creating role clone for ${component}"
  _mapfile role_unit_labels < <(label_args role "${component}" --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}")
  create_clone_unit "$(role_space)" "$(unit_name "${component}" role)" "$(region_space)" "$(unit_name "${component}" region)" "${role_unit_labels[@]}"
  apply_role_mutations "${component}"

  echo "==> Creating recipe clone for ${component}"
  _mapfile recipe_unit_labels < <(label_args recipe "${component}" --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}")
  create_clone_unit "$(recipe_space)" "$(unit_name "${component}" recipe)" "$(role_space)" "$(unit_name "${component}" role)" "${recipe_unit_labels[@]}"
  apply_recipe_mutations "${component}"

  echo "==> Creating deployment clone for ${component}"
  _mapfile deploy_unit_labels < <(label_args deployment "${component}" --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}")
  create_clone_unit "$(deploy_space)" "$(unit_name "${component}" deployment)" "$(recipe_space)" "$(unit_name "${component}" recipe)" "${deploy_unit_labels[@]}"
  apply_deploy_mutations "${component}"
done

echo "==> Creating backend stub (dependency for frontend)"
_mapfile deploy_stub_labels < <(label_args deployment backend-stub --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}")
create_unit_from_file "$(deploy_space)" "${DEPLOY_STUB_UNIT}" "${BACKEND_STUB_YAML}" "${deploy_stub_labels[@]}"
cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "${DEPLOY_STUB_UNIT}"

if [[ -n "${TARGET_REF}" ]]; then
  echo "==> Setting target on deployment units"
  set_target_for_deploy_units "${TARGET_REF}"
  cub unit set-target "${TARGET_REF}" --space "$(deploy_space)" --unit "${DEPLOY_STUB_UNIT}"
fi

echo "==> Rendering explicit app-level recipe manifest"
refresh_recipe_manifest_unit "${TARGET_REF}"

show_summary "${TARGET_REF}"
