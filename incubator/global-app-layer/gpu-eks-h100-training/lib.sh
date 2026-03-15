#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="${SCRIPT_DIR}/.state"
STATE_FILE="${STATE_DIR}/state.env"
RECIPE_BASE_TEMPLATE="${SCRIPT_DIR}/recipe.base.yaml"
BASE_MANIFEST="${SCRIPT_DIR}/gpu-operator.base.yaml"

EXAMPLE_NAME="global-app-layer-gpu-eks-h100-training"
CHAIN_LABEL="gpu-operator-eks-h100-ubuntu-training"
COMPONENT_NAME="gpu-operator"

BASE_SPACE_SUFFIX="catalog-base"
PLATFORM_SPACE_SUFFIX="catalog-eks"
ACCELERATOR_SPACE_SUFFIX="catalog-h100"
OS_SPACE_SUFFIX="catalog-ubuntu"
RECIPE_SPACE_SUFFIX="recipe-eks-h100-ubuntu-training"
DEPLOY_SPACE_SUFFIX="deploy-cluster-a"

RECIPE_MANIFEST_UNIT="recipe-eks-h100-ubuntu-training"
PLATFORM_VALUE="eks"
ACCELERATOR_VALUE="h100"
OS_VALUE="ubuntu"
INTENT_VALUE="training"
DEPLOY_NAMESPACE="cluster-a"
DEFAULT_OPERATOR_TAG="24.6.1"

_mapfile() {
  local _var="$1"
  eval "${_var}=()"
  while IFS= read -r _line; do
    eval "${_var}+=(\"\${_line}\")"
  done
}

require_cub() {
  if ! command -v cub >/dev/null 2>&1; then
    echo "Missing required command: cub" >&2
    exit 1
  fi
}

require_jq() {
  if ! command -v jq >/dev/null 2>&1; then
    echo "Missing required command: jq" >&2
    exit 1
  fi
}

ensure_state_dir() {
  mkdir -p "${STATE_DIR}"
}

state_exists() {
  [[ -f "${STATE_FILE}" ]]
}

load_state() {
  if [[ ! -f "${STATE_FILE}" ]]; then
    echo "No state file found. Run ./setup.sh first." >&2
    exit 1
  fi
  # shellcheck disable=SC1090
  source "${STATE_FILE}"
}

save_state() {
  local prefix="$1"
  local target_ref="${2:-}"

  ensure_state_dir
  printf 'PREFIX=%q\nTARGET_REF=%q\n' "${prefix}" "${target_ref}" >"${STATE_FILE}"
}

state_prefix() {
  echo "${PREFIX:?missing PREFIX in state}"
}

space_name() {
  local suffix="$1"
  echo "$(state_prefix)-${suffix}"
}

base_space() { space_name "${BASE_SPACE_SUFFIX}"; }
platform_space() { space_name "${PLATFORM_SPACE_SUFFIX}"; }
accelerator_space() { space_name "${ACCELERATOR_SPACE_SUFFIX}"; }
os_space() { space_name "${OS_SPACE_SUFFIX}"; }
recipe_space() { space_name "${RECIPE_SPACE_SUFFIX}"; }
deploy_space() { space_name "${DEPLOY_SPACE_SUFFIX}"; }

unit_name() {
  local stage="$1"
  case "${stage}" in
    base) echo "${COMPONENT_NAME}-base" ;;
    platform) echo "${COMPONENT_NAME}-eks" ;;
    accelerator) echo "${COMPONENT_NAME}-eks-h100" ;;
    os) echo "${COMPONENT_NAME}-eks-h100-ubuntu" ;;
    recipe) echo "${COMPONENT_NAME}-eks-h100-ubuntu-training" ;;
    deployment) echo "${COMPONENT_NAME}-cluster-a" ;;
    *)
      echo "Unknown stage: ${stage}" >&2
      exit 1
      ;;
  esac
}

space_exists() {
  local space="$1"
  cub space get "${space}" >/dev/null 2>&1
}

unit_exists() {
  local space="$1"
  local unit="$2"
  cub unit get --space "${space}" "${unit}" >/dev/null 2>&1
}

create_space_if_missing() {
  local space="$1"
  shift
  if space_exists "${space}"; then
    echo "Space already exists: ${space}"
    return
  fi
  cub space create "${space}" "$@"
}

create_unit_from_file() {
  local space="$1"
  local unit="$2"
  local file_path="$3"
  shift 3
  if unit_exists "${space}" "${unit}"; then
    echo "Unit already exists: ${space}/${unit}"
    return
  fi
  cub unit create --space "${space}" "${unit}" "${file_path}" "$@"
}

create_clone_unit() {
  local space="$1"
  local unit="$2"
  local upstream_space="$3"
  local upstream_unit="$4"
  shift 4
  if unit_exists "${space}" "${unit}"; then
    echo "Unit already exists: ${space}/${unit}"
    return
  fi
  cub unit create --space "${space}" "${unit}" \
    --upstream-unit "${upstream_unit}" \
    --upstream-space "${upstream_space}" \
    "$@"
}

space_label_args() {
  local layer_kind="$1"
  shift
  printf '%s\n' \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "ExampleChain=$(state_prefix)" \
    --label "Recipe=${CHAIN_LABEL}" \
    --label "Component=${COMPONENT_NAME}" \
    --label "LayerKind=${layer_kind}" \
    "$@"
}

label_args() {
  local layer="$1"
  shift
  printf '%s\n' \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "ExampleChain=$(state_prefix)" \
    --label "Recipe=${CHAIN_LABEL}" \
    --label "Component=${COMPONENT_NAME}" \
    --label "Layer=${layer}" \
    "$@"
}

assert_contains() {
  local file_path="$1"
  local pattern="$2"
  if ! grep -q --fixed-strings "${pattern}" "${file_path}"; then
    echo "Expected pattern not found in ${file_path}: ${pattern}" >&2
    exit 1
  fi
}

unit_data_to_file() {
  local space="$1"
  local unit="$2"
  local output_path="$3"
  cub unit get --space "${space}" --data-only "${unit}" >"${output_path}"
}

get_unit_json() {
  local space="$1"
  local unit="$2"
  cub unit get --space "${space}" --json "${unit}"
}

get_unit_field() {
  local space="$1"
  local unit="$2"
  local field_path="$3"
  get_unit_json "${space}" "${unit}" | jq -e -r ".Unit.${field_path}"
}

bundle_hint_from_target_ref() {
  local target_ref="$1"
  if [[ -z "${target_ref}" ]]; then
    echo "set a target to compute the target bundle path"
    return
  fi
  if [[ "${target_ref}" == */* ]]; then
    local target_space="${target_ref%%/*}"
    local target_slug="${target_ref##*/}"
    echo "target/${target_space}/${target_slug}:latest"
    return
  fi
  echo "target/<target-space>/${target_ref}:latest"
}

render_recipe_manifest() {
  local output_path="$1"
  local target_ref="$2"
  local effective_target_ref="${target_ref:-unset}"

  local base_rev platform_rev accelerator_rev os_rev recipe_rev deploy_rev
  local base_hash platform_hash accelerator_hash os_hash recipe_hash deploy_hash

  base_rev="$(get_unit_field "$(base_space)" "$(unit_name base)" HeadRevisionNum)"
  platform_rev="$(get_unit_field "$(platform_space)" "$(unit_name platform)" HeadRevisionNum)"
  accelerator_rev="$(get_unit_field "$(accelerator_space)" "$(unit_name accelerator)" HeadRevisionNum)"
  os_rev="$(get_unit_field "$(os_space)" "$(unit_name os)" HeadRevisionNum)"
  recipe_rev="$(get_unit_field "$(recipe_space)" "$(unit_name recipe)" HeadRevisionNum)"
  deploy_rev="$(get_unit_field "$(deploy_space)" "$(unit_name deployment)" HeadRevisionNum)"

  base_hash="$(get_unit_field "$(base_space)" "$(unit_name base)" DataHash || true)"
  platform_hash="$(get_unit_field "$(platform_space)" "$(unit_name platform)" DataHash || true)"
  accelerator_hash="$(get_unit_field "$(accelerator_space)" "$(unit_name accelerator)" DataHash || true)"
  os_hash="$(get_unit_field "$(os_space)" "$(unit_name os)" DataHash || true)"
  recipe_hash="$(get_unit_field "$(recipe_space)" "$(unit_name recipe)" DataHash || true)"
  deploy_hash="$(get_unit_field "$(deploy_space)" "$(unit_name deployment)" DataHash || true)"

  sed \
    -e "s|confighubplaceholder-chain-name|${CHAIN_LABEL}|g" \
    -e "s|confighubplaceholder-example-name|${EXAMPLE_NAME}|g" \
    -e "s|confighubplaceholder-chain-prefix|$(state_prefix)|g" \
    -e "s|confighubplaceholder-target-ref|${effective_target_ref}|g" \
    -e "s|confighubplaceholder-bundle-hint|$(bundle_hint_from_target_ref "${target_ref}")|g" \
    -e "s|confighubplaceholder-base-space|$(base_space)|g" \
    -e "s|confighubplaceholder-base-unit|$(unit_name base)|g" \
    -e "s|confighubplaceholder-base-revision|${base_rev}|g" \
    -e "s|confighubplaceholder-base-hash|${base_hash}|g" \
    -e "s|confighubplaceholder-platform-space|$(platform_space)|g" \
    -e "s|confighubplaceholder-platform-unit|$(unit_name platform)|g" \
    -e "s|confighubplaceholder-platform-revision|${platform_rev}|g" \
    -e "s|confighubplaceholder-platform-hash|${platform_hash}|g" \
    -e "s|confighubplaceholder-accelerator-space|$(accelerator_space)|g" \
    -e "s|confighubplaceholder-accelerator-unit|$(unit_name accelerator)|g" \
    -e "s|confighubplaceholder-accelerator-revision|${accelerator_rev}|g" \
    -e "s|confighubplaceholder-accelerator-hash|${accelerator_hash}|g" \
    -e "s|confighubplaceholder-os-space|$(os_space)|g" \
    -e "s|confighubplaceholder-os-unit|$(unit_name os)|g" \
    -e "s|confighubplaceholder-os-revision|${os_rev}|g" \
    -e "s|confighubplaceholder-os-hash|${os_hash}|g" \
    -e "s|confighubplaceholder-recipe-space|$(recipe_space)|g" \
    -e "s|confighubplaceholder-recipe-unit|$(unit_name recipe)|g" \
    -e "s|confighubplaceholder-recipe-revision|${recipe_rev}|g" \
    -e "s|confighubplaceholder-recipe-hash|${recipe_hash}|g" \
    -e "s|confighubplaceholder-deploy-space|$(deploy_space)|g" \
    -e "s|confighubplaceholder-deploy-unit|$(unit_name deployment)|g" \
    -e "s|confighubplaceholder-deploy-revision|${deploy_rev}|g" \
    -e "s|confighubplaceholder-deploy-hash|${deploy_hash}|g" \
    "${RECIPE_BASE_TEMPLATE}" >"${output_path}"
}

refresh_recipe_manifest_unit() {
  local target_ref="$1"
  local rendered_manifest="${STATE_DIR}/recipe-eks-h100-ubuntu-training.rendered.yaml"
  ensure_state_dir
  render_recipe_manifest "${rendered_manifest}" "${target_ref}"

  if unit_exists "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}"; then
    cub unit update --space "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}" "${rendered_manifest}"
  else
    _mapfile manifest_labels < <(label_args recipe-manifest)
    cub unit create --space "$(recipe_space)" -t AppConfig/YAML \
      "${RECIPE_MANIFEST_UNIT}" "${rendered_manifest}" "${manifest_labels[@]}"
  fi
}

apply_platform_mutations() {
  cub function do set-env gpu-operator "CLOUD_PROVIDER=${PLATFORM_VALUE}" --space "$(platform_space)" --unit "$(unit_name platform)"
  cub function do set-env gpu-operator "STORAGE_CLASS=gp3" --space "$(platform_space)" --unit "$(unit_name platform)"
}

apply_accelerator_mutations() {
  cub function do set-env gpu-operator "ACCELERATOR=${ACCELERATOR_VALUE}" --space "$(accelerator_space)" --unit "$(unit_name accelerator)"
  cub function do set-env gpu-operator "NODE_SELECTOR=nvidia-h100" --space "$(accelerator_space)" --unit "$(unit_name accelerator)"
}

apply_os_mutations() {
  cub function do set-env gpu-operator "OS_FAMILY=${OS_VALUE}" --space "$(os_space)" --unit "$(unit_name os)"
  cub function do set-env gpu-operator "DRIVER_BRANCH=550-ubuntu22.04" --space "$(os_space)" --unit "$(unit_name os)"
}

apply_recipe_mutations() {
  cub function do set-env gpu-operator "WORKLOAD_INTENT=${INTENT_VALUE}" --space "$(recipe_space)" --unit "$(unit_name recipe)"
  cub function do set-env gpu-operator "VALIDATION_PROFILE=training-smoke" --space "$(recipe_space)" --unit "$(unit_name recipe)"
}

apply_deploy_mutations() {
  cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "$(unit_name deployment)"
  cub function do set-env gpu-operator "CLUSTER=${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "$(unit_name deployment)"
}

set_target_for_deploy_unit() {
  local target_ref="$1"
  cub unit set-target "${target_ref}" --space "$(deploy_space)" --unit "$(unit_name deployment)"
}

show_summary() {
  local target_ref="$1"
  cat <<EOF_SUMMARY
Created GPU layered chain with prefix: $(state_prefix)

Spaces:
- $(base_space)
- $(platform_space)
- $(accelerator_space)
- $(os_space)
- $(recipe_space)
- $(deploy_space)

Units:
- $(base_space)/$(unit_name base)
- $(platform_space)/$(unit_name platform)
- $(accelerator_space)/$(unit_name accelerator)
- $(os_space)/$(unit_name os)
- $(recipe_space)/$(unit_name recipe)
- $(deploy_space)/$(unit_name deployment)
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_OPERATOR_TAG}
3. ${target_ref:+cub unit approve --space $(deploy_space) $(unit_name deployment)}
4. ${target_ref:+cub unit apply --space $(deploy_space) $(unit_name deployment)}
5. ${target_ref:+Review recipe manifest: cub unit get --space $(recipe_space) --data-only ${RECIPE_MANIFEST_UNIT}}
${target_ref:-3. ./set-target.sh <space/target>  # optional, enables apply + target bundle story}
EOF_SUMMARY
}
