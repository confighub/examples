#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

mode="apply"
positionals=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain)
      mode="explain"
      shift
      ;;
    --explain-json)
      mode="explain-json"
      shift
      ;;
    -h|--help)
      setup_usage
      exit 0
      ;;
    --)
      shift
      while [[ $# -gt 0 ]]; do
        positionals+=("$1")
        shift
      done
      ;;
    -*)
      echo "Unknown flag: $1" >&2
      setup_usage >&2
      exit 1
      ;;
    *)
      positionals+=("$1")
      shift
      ;;
  esac
done

if (( ${#positionals[@]} > 2 )); then
  echo "Too many positional arguments. Expected [prefix] [space/target]." >&2
  setup_usage >&2
  exit 1
fi

prefix="${positionals[0]:-}"
target_ref="${positionals[1]:-}"

if [[ "${mode}" != "apply" ]]; then
  prefix_source="provided"
  if [[ -z "${prefix}" ]]; then
    prefix="<generated-prefix>"
    prefix_source="generated-at-run-time"
  fi
  PREFIX="${prefix}"
  TARGET_REF="${target_ref}"

  case "${mode}" in
    explain)
      print_setup_explain "${prefix_source}"
      ;;
    explain-json)
      require_jq
      print_setup_explain_json "${prefix_source}"
      ;;
  esac
  exit 0
fi

require_cub
require_jq
begin_log_capture setup

if state_exists; then
  echo "State already exists in ${STATE_FILE}. The .state directory is local run state from an earlier setup. Run ./cleanup.sh first, or remove .state if you know this old state is no longer needed." >&2
  exit 1
fi

if [[ -z "${prefix}" ]]; then
  prefix="$(cub space new-prefix)"
fi

assert_supported_live_target "${target_ref}"

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

echo "==> Creating deployment bootstrap namespace unit"
ensure_namespace_unit "${target_ref}"

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

if [[ -n "${TARGET_REF}" ]]; then
  echo "==> Setting target on deployment clones"
  set_target_for_deploy_units "${TARGET_REF}"
fi

echo "==> Rendering explicit app-level recipe manifest"
refresh_recipe_manifest_unit "${TARGET_REF}"

show_summary "${TARGET_REF}"
