#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="${SCRIPT_DIR}/.state"
STATE_FILE="${STATE_DIR}/state.env"
RECIPE_BASE_TEMPLATE="${SCRIPT_DIR}/recipe.base.yaml"
SOURCE_BACKEND_YAML="${SCRIPT_DIR}/../../../global-app/baseconfig/backend.yaml"

EXAMPLE_NAME="global-app-layer-single"
CHAIN_LABEL="global-app-us-staging"
COMPONENT="backend"

BASE_SPACE_SUFFIX="catalog-base"
REGION_SPACE_SUFFIX="catalog-us"
ROLE_SPACE_SUFFIX="catalog-us-staging"
RECIPE_SPACE_SUFFIX="recipe-us-staging"
DEPLOY_SPACE_SUFFIX="deploy-cluster-a"

BASE_UNIT="backend-base"
REGION_UNIT="backend-us"
ROLE_UNIT="backend-us-staging"
RECIPE_UNIT="backend-recipe-us-staging"
DEPLOY_UNIT="backend-cluster-a"
RECIPE_MANIFEST_UNIT="recipe-us-staging"

REGION_VALUE="US"
ROLE_VALUE="staging"
DEPLOY_NAMESPACE="cluster-a"
DEFAULT_IMAGE_TAG="1.1.8"

# bash 3-compatible replacement for mapfile -t
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

base_space() {
  space_name "${BASE_SPACE_SUFFIX}"
}

region_space() {
  space_name "${REGION_SPACE_SUFFIX}"
}

role_space() {
  space_name "${ROLE_SPACE_SUFFIX}"
}

recipe_space() {
  space_name "${RECIPE_SPACE_SUFFIX}"
}

deploy_space() {
  space_name "${DEPLOY_SPACE_SUFFIX}"
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

label_args() {
  local layer="$1"
  shift
  printf '%s\n' \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "ExampleChain=$(state_prefix)" \
    --label "Recipe=${CHAIN_LABEL}" \
    --label "Component=${COMPONENT}" \
    --label "Layer=${layer}" \
    "$@"
}

space_label_args() {
  local layer_kind="$1"
  shift
  printf '%s\n' \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "ExampleChain=$(state_prefix)" \
    --label "Recipe=${CHAIN_LABEL}" \
    --label "LayerKind=${layer_kind}" \
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

  local base_rev region_rev role_rev recipe_rev deploy_rev
  local base_hash region_hash role_hash recipe_hash deploy_hash
  base_rev="$(get_unit_field "$(base_space)" "${BASE_UNIT}" HeadRevisionNum)"
  region_rev="$(get_unit_field "$(region_space)" "${REGION_UNIT}" HeadRevisionNum)"
  role_rev="$(get_unit_field "$(role_space)" "${ROLE_UNIT}" HeadRevisionNum)"
  recipe_rev="$(get_unit_field "$(recipe_space)" "${RECIPE_UNIT}" HeadRevisionNum)"
  deploy_rev="$(get_unit_field "$(deploy_space)" "${DEPLOY_UNIT}" HeadRevisionNum)"

  base_hash="$(get_unit_field "$(base_space)" "${BASE_UNIT}" DataHash || true)"
  region_hash="$(get_unit_field "$(region_space)" "${REGION_UNIT}" DataHash || true)"
  role_hash="$(get_unit_field "$(role_space)" "${ROLE_UNIT}" DataHash || true)"
  recipe_hash="$(get_unit_field "$(recipe_space)" "${RECIPE_UNIT}" DataHash || true)"
  deploy_hash="$(get_unit_field "$(deploy_space)" "${DEPLOY_UNIT}" DataHash || true)"

  sed \
    -e "s|confighubplaceholder-chain-name|${CHAIN_LABEL}|g" \
    -e "s|confighubplaceholder-example-name|${EXAMPLE_NAME}|g" \
    -e "s|confighubplaceholder-chain-prefix|$(state_prefix)|g" \
    -e "s|confighubplaceholder-base-space|$(base_space)|g" \
    -e "s|confighubplaceholder-base-unit|${BASE_UNIT}|g" \
    -e "s|confighubplaceholder-base-revision|${base_rev}|g" \
    -e "s|confighubplaceholder-base-hash|${base_hash}|g" \
    -e "s|confighubplaceholder-region-space|$(region_space)|g" \
    -e "s|confighubplaceholder-region-unit|${REGION_UNIT}|g" \
    -e "s|confighubplaceholder-region-revision|${region_rev}|g" \
    -e "s|confighubplaceholder-region-hash|${region_hash}|g" \
    -e "s|confighubplaceholder-role-space|$(role_space)|g" \
    -e "s|confighubplaceholder-role-unit|${ROLE_UNIT}|g" \
    -e "s|confighubplaceholder-role-revision|${role_rev}|g" \
    -e "s|confighubplaceholder-role-hash|${role_hash}|g" \
    -e "s|confighubplaceholder-recipe-space|$(recipe_space)|g" \
    -e "s|confighubplaceholder-recipe-unit|${RECIPE_UNIT}|g" \
    -e "s|confighubplaceholder-recipe-revision|${recipe_rev}|g" \
    -e "s|confighubplaceholder-recipe-hash|${recipe_hash}|g" \
    -e "s|confighubplaceholder-deploy-space|$(deploy_space)|g" \
    -e "s|confighubplaceholder-deploy-unit|${DEPLOY_UNIT}|g" \
    -e "s|confighubplaceholder-deploy-revision|${deploy_rev}|g" \
    -e "s|confighubplaceholder-deploy-hash|${deploy_hash}|g" \
    -e "s|confighubplaceholder-target-ref|${effective_target_ref}|g" \
    -e "s|confighubplaceholder-bundle-hint|$(bundle_hint_from_target_ref "${target_ref}")|g" \
    "${RECIPE_BASE_TEMPLATE}" >"${output_path}"
}

refresh_recipe_manifest_unit() {
  local target_ref="$1"
  local rendered_manifest="${STATE_DIR}/recipe-us-staging.rendered.yaml"
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

show_summary() {
  local target_ref="$1"
  cat <<EOF_SUMMARY
Created global-app layered chain with prefix: $(state_prefix)

Spaces:
- $(base_space)
- $(region_space)
- $(role_space)
- $(recipe_space)
- $(deploy_space)

Units:
- $(base_space)/${BASE_UNIT}
- $(region_space)/${REGION_UNIT}
- $(role_space)/${ROLE_UNIT}
- $(recipe_space)/${RECIPE_UNIT}
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}
- $(deploy_space)/${DEPLOY_UNIT}

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_IMAGE_TAG}
3. ${target_ref:+cub unit approve --space $(deploy_space) ${DEPLOY_UNIT} && cub unit apply --space $(deploy_space) ${DEPLOY_UNIT}}
4. ${target_ref:+Review recipe manifest: cub unit get --space $(recipe_space) --data-only ${RECIPE_MANIFEST_UNIT}}
${target_ref:-3. ./set-target.sh <space/target>  # optional, enables apply + target bundle story}
EOF_SUMMARY
}
