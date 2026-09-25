#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

mode="run"
case "${1:-}" in
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
  -*)
    echo "Unknown flag: ${1}" >&2
    setup_usage >&2
    exit 1
    ;;
esac

prefix="${1:-}"
shift_count=0
if [[ -n "${prefix}" ]]; then
  shift_count=1
fi
if [[ "${shift_count}" -gt 0 ]]; then
  shift
fi
target_refs=("$@")

if [[ "${mode}" != "run" ]]; then
  PREFIX="${prefix:-<generated-prefix>}"
  DIRECT_TARGET_REF=""
  FLUX_TARGET_REF=""
  ARGO_TARGET_REF=""
  if [[ "${mode}" == "explain-json" ]]; then
    require_jq
    if [[ "${#target_refs[@]}" -gt 0 ]]; then
      show_setup_plan_json "${target_refs[@]}"
    else
      show_setup_plan_json
    fi
  else
    if [[ "${#target_refs[@]}" -gt 0 ]]; then
      show_setup_plan "${target_refs[@]}"
    else
      show_setup_plan
    fi
  fi
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

if [[ "${#target_refs[@]}" -gt 0 ]]; then
  for target_ref in "${target_refs[@]}"; do
    assert_supported_live_target "${target_ref}"
  done
fi

save_state "${prefix}" "" "" ""
load_state

_mapfile base_space_labels < <(space_label_args base)
_mapfile platform_space_labels < <(space_label_args platform --label "Platform=${PLATFORM_VALUE}")
_mapfile accelerator_space_labels < <(space_label_args accelerator --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}")
_mapfile os_space_labels < <(space_label_args os --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}")
_mapfile recipe_space_labels < <(space_label_args recipe --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}")
_mapfile direct_deploy_space_labels < <(space_label_args deployment --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=direct")
_mapfile flux_deploy_space_labels < <(space_label_args deployment --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=flux")
_mapfile argo_deploy_space_labels < <(space_label_args deployment --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=argo")

echo "==> Creating spaces"
create_space_if_missing "$(base_space)" "${base_space_labels[@]}"
create_space_if_missing "$(platform_space)" "${platform_space_labels[@]}"
create_space_if_missing "$(accelerator_space)" "${accelerator_space_labels[@]}"
create_space_if_missing "$(os_space)" "${os_space_labels[@]}"
create_space_if_missing "$(recipe_space)" "${recipe_space_labels[@]}"
create_space_if_missing "$(deploy_space)" "${direct_deploy_space_labels[@]}"
create_space_if_missing "$(flux_deploy_space)" "${flux_deploy_space_labels[@]}"
create_space_if_missing "$(argo_deploy_space)" "${argo_deploy_space_labels[@]}"

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

  echo "==> Creating direct deployment clone for ${component}"
  _mapfile direct_deploy_unit_labels < <(label_args deployment "${component}" --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=direct")
  create_clone_unit "$(deploy_space)" "$(deployment_unit_name "${component}" direct)" "$(recipe_space)" "$(unit_name "${component}" recipe)" "${direct_deploy_unit_labels[@]}"
  apply_deploy_mutations "${component}" direct

  echo "==> Creating Flux deployment clone for ${component}"
  _mapfile flux_deploy_unit_labels < <(label_args deployment "${component}" --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=flux")
  create_clone_unit "$(flux_deploy_space)" "$(deployment_unit_name "${component}" flux)" "$(recipe_space)" "$(unit_name "${component}" recipe)" "${flux_deploy_unit_labels[@]}"
  apply_deploy_mutations "${component}" flux

  echo "==> Creating Argo deployment clone for ${component}"
  _mapfile argo_deploy_unit_labels < <(label_args deployment "${component}" --label "Platform=${PLATFORM_VALUE}" --label "Accelerator=${ACCELERATOR_VALUE}" --label "OS=${OS_VALUE}" --label "Intent=${INTENT_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=argo")
  create_clone_unit "$(argo_deploy_space)" "$(deployment_unit_name "${component}" argo)" "$(recipe_space)" "$(unit_name "${component}" recipe)" "${argo_deploy_unit_labels[@]}"
  apply_deploy_mutations "${component}" argo
done

if [[ "${#target_refs[@]}" -gt 0 ]]; then
  echo "==> Setting targets on compatible deployment variants"
  for target_ref in "${target_refs[@]}"; do
    set_target_for_compatible_units "${target_ref}"
  done
  save_state "${PREFIX}" "${DIRECT_TARGET_REF:-}" "${FLUX_TARGET_REF:-}" "${ARGO_TARGET_REF:-}"
fi

echo "==> Rendering explicit GPU recipe manifest"
refresh_recipe_manifest_unit "${DIRECT_TARGET_REF:-}" "${FLUX_TARGET_REF:-}" "${ARGO_TARGET_REF:-}"

show_summary
