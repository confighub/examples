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

# Validate any provided targets before starting
DIRECT_TARGET_REF=""
FLUX_TARGET_REF=""
ARGO_TARGET_REF=""
for target_ref in "${target_refs[@]}"; do
  if [[ -n "${target_ref}" ]]; then
    assert_supported_live_target "${target_ref}"
  fi
done

save_state "${prefix}" "${DIRECT_TARGET_REF}" "${FLUX_TARGET_REF}" "${ARGO_TARGET_REF}"
load_state

_mapfile base_space_labels < <(space_label_args base)
_mapfile region_space_labels < <(space_label_args region --label "Region=${REGION_VALUE}")
_mapfile role_space_labels < <(space_label_args role --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}")
_mapfile recipe_space_labels < <(space_label_args recipe --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}")
_mapfile deploy_space_labels < <(space_label_args deployment --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=direct")
_mapfile flux_deploy_space_labels < <(space_label_args deployment --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=flux")
_mapfile argo_deploy_space_labels < <(space_label_args deployment --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=argo")

echo "==> Creating spaces"
create_space_if_missing "$(base_space)" "${base_space_labels[@]}"
create_space_if_missing "$(region_space)" "${region_space_labels[@]}"
create_space_if_missing "$(role_space)" "${role_space_labels[@]}"
create_space_if_missing "$(recipe_space)" "${recipe_space_labels[@]}"
create_space_if_missing "$(deploy_space)" "${deploy_space_labels[@]}"
create_space_if_missing "$(flux_deploy_space)" "${flux_deploy_space_labels[@]}"
create_space_if_missing "$(argo_deploy_space)" "${argo_deploy_space_labels[@]}"

_mapfile base_unit_labels < <(label_args base)
_mapfile region_unit_labels < <(label_args region --label "Region=${REGION_VALUE}")
_mapfile role_unit_labels < <(label_args role --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}")
_mapfile recipe_unit_labels < <(label_args recipe --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}")
_mapfile deploy_unit_labels < <(label_args deployment --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=direct")
_mapfile flux_deploy_unit_labels < <(label_args deployment --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=flux")
_mapfile argo_deploy_unit_labels < <(label_args deployment --label "Region=${REGION_VALUE}" --label "Role=${ROLE_VALUE}" --label "Cluster=${DEPLOY_NAMESPACE}" --label "DeliveryVariant=argo")

echo "==> Creating base unit from global-app/baseconfig/backend.yaml"
create_unit_from_file "$(base_space)" "${BASE_UNIT}" "${SOURCE_BACKEND_YAML}" "${base_unit_labels[@]}"

echo "==> Creating region clone"
create_clone_unit "$(region_space)" "${REGION_UNIT}" "$(base_space)" "${BASE_UNIT}" "${region_unit_labels[@]}"
cub function do set-env backend "REGION=${REGION_VALUE}" --space "$(region_space)" --unit "${REGION_UNIT}"
cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host backend.us.demo.confighub.local --space "$(region_space)" --unit "${REGION_UNIT}"

echo "==> Creating role clone"
create_clone_unit "$(role_space)" "${ROLE_UNIT}" "$(region_space)" "${REGION_UNIT}" "${role_unit_labels[@]}"
cub function do set-env backend "ROLE=${ROLE_VALUE}" --space "$(role_space)" --unit "${ROLE_UNIT}"
cub function do set-replicas 2 --space "$(role_space)" --unit "${ROLE_UNIT}"
cub function do set-env backend "LOG_LEVEL=info" --space "$(role_space)" --unit "${ROLE_UNIT}"

echo "==> Creating recipe clone"
create_clone_unit "$(recipe_space)" "${RECIPE_UNIT}" "$(role_space)" "${ROLE_UNIT}" "${recipe_unit_labels[@]}"
cub function do set-env backend "CHAT_TITLE=Cubby Chat (US Staging Recipe)" --space "$(recipe_space)" --unit "${RECIPE_UNIT}"

echo "==> Creating direct deployment clone"
create_clone_unit "$(deploy_space)" "${DEPLOY_UNIT}" "$(recipe_space)" "${RECIPE_UNIT}" "${deploy_unit_labels[@]}"
cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "${DEPLOY_UNIT}"
cub function do set-env backend "CLUSTER=${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "${DEPLOY_UNIT}"
cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host "$(deploy_backend_hostname)" --space "$(deploy_space)" --unit "${DEPLOY_UNIT}"

echo "==> Creating Flux deployment clone"
create_clone_unit "$(flux_deploy_space)" "${DEPLOY_FLUX_UNIT}" "$(recipe_space)" "${RECIPE_UNIT}" "${flux_deploy_unit_labels[@]}"
cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(flux_deploy_space)" --unit "${DEPLOY_FLUX_UNIT}"
cub function do set-env backend "CLUSTER=${DEPLOY_NAMESPACE}" --space "$(flux_deploy_space)" --unit "${DEPLOY_FLUX_UNIT}"
cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host "$(deploy_backend_hostname)" --space "$(flux_deploy_space)" --unit "${DEPLOY_FLUX_UNIT}"

echo "==> Creating Argo deployment clone"
create_clone_unit "$(argo_deploy_space)" "${DEPLOY_ARGO_UNIT}" "$(recipe_space)" "${RECIPE_UNIT}" "${argo_deploy_unit_labels[@]}"
cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(argo_deploy_space)" --unit "${DEPLOY_ARGO_UNIT}"
cub function do set-env backend "CLUSTER=${DEPLOY_NAMESPACE}" --space "$(argo_deploy_space)" --unit "${DEPLOY_ARGO_UNIT}"
cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host "$(deploy_backend_hostname)" --space "$(argo_deploy_space)" --unit "${DEPLOY_ARGO_UNIT}"

echo "==> Creating postgres stub (direct variant)"
create_unit_from_file "$(deploy_space)" "${DEPLOY_STUB_UNIT}" "${POSTGRES_STUB_YAML}" "${deploy_unit_labels[@]}"
cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "${DEPLOY_STUB_UNIT}"

echo "==> Creating postgres stub (flux variant)"
create_unit_from_file "$(flux_deploy_space)" "postgres-stub-flux" "${POSTGRES_STUB_YAML}" "${flux_deploy_unit_labels[@]}"
cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(flux_deploy_space)" --unit "postgres-stub-flux"

echo "==> Creating postgres stub (argo variant)"
create_unit_from_file "$(argo_deploy_space)" "postgres-stub-argo" "${POSTGRES_STUB_YAML}" "${argo_deploy_unit_labels[@]}"
cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(argo_deploy_space)" --unit "postgres-stub-argo"

# Set targets based on provider type
for target_ref in "${target_refs[@]}"; do
  if [[ -n "${target_ref}" ]]; then
    echo "==> Setting target ${target_ref}"
    set_target_for_compatible_units "${target_ref}"
  fi
done

save_state "${PREFIX}" "${DIRECT_TARGET_REF:-}" "${FLUX_TARGET_REF:-}" "${ARGO_TARGET_REF:-}"

echo "==> Rendering explicit recipe manifest"
refresh_recipe_manifest_unit "${DIRECT_TARGET_REF:-}" "${FLUX_TARGET_REF:-}" "${ARGO_TARGET_REF:-}"

show_summary
