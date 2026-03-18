#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="${SCRIPT_DIR}/.state"
STATE_FILE="${STATE_DIR}/state.env"
LOG_DIR="${SCRIPT_DIR}/.logs"
RECIPE_BASE_TEMPLATE="${SCRIPT_DIR}/recipe.base.yaml"
SOURCE_BACKEND_YAML="${SCRIPT_DIR}/../../../global-app/baseconfig/backend.yaml"
POSTGRES_STUB_YAML="${SCRIPT_DIR}/postgres-stub.yaml"

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
DEPLOY_NAMESPACE="${DEPLOY_NAMESPACE:-cluster-a}"
DEPLOY_STUB_UNIT="postgres-stub"
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

ensure_log_dir() {
  mkdir -p "${LOG_DIR}"
}

current_log_path() {
  local script_name="${1:-$(basename "$0" .sh)}"
  echo "${LOG_DIR}/${script_name}.latest.log"
}

begin_log_capture() {
  local script_name="${1:-$(basename "$0" .sh)}"
  if [[ "${CONFIGHUB_EXAMPLE_LOG_CAPTURED:-}" == "1" ]]; then
    return
  fi
  ensure_log_dir
  export CONFIGHUB_EXAMPLE_LOG_CAPTURED=1
  exec > >(tee "$(current_log_path "${script_name}")") 2>&1
  echo "Log file: $(current_log_path "${script_name}")"
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
  printf 'PREFIX=%q\nTARGET_REF=%q\nDEPLOY_NAMESPACE=%q\n' "${prefix}" "${target_ref}" "${DEPLOY_NAMESPACE}" >"${STATE_FILE}"
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

get_space_json() {
  local space="$1"
  cub space get --json "${space}"
}

get_space_field() {
  local space="$1"
  local field_path="$2"

  get_space_json "${space}" | jq -e -r ".Space.${field_path}"
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

current_server_url() {
  local url
  url="$(cub context list 2>/dev/null | awk '$1 == "*" {print $3; exit}')"
  if [[ -n "${url}" ]]; then
    echo "${url}"
    return
  fi
  url="$(cub version 2>/dev/null | awk '/URL:/ {print $2; exit}')"
  if [[ -n "${url}" ]]; then
    echo "${url}"
  fi
}

gui_space_url() {
  local space="$1"
  local server_url space_id
  server_url="$(current_server_url)"
  space_id="$(get_space_field "${space}" SpaceID 2>/dev/null || true)"
  if [[ -n "${server_url}" && -n "${space_id}" ]]; then
    echo "${server_url}/spaces/${space_id}"
  fi
}

gui_unit_url() {
  local space="$1"
  local unit="$2"
  local server_url space_id unit_id
  server_url="$(current_server_url)"
  space_id="$(get_space_field "${space}" SpaceID 2>/dev/null || true)"
  unit_id="$(get_unit_field "${space}" "${unit}" UnitID 2>/dev/null || true)"
  if [[ -n "${server_url}" && -n "${space_id}" && -n "${unit_id}" ]]; then
    echo "${server_url}/units/${space_id}/${unit_id}"
  fi
}

show_setup_plan() {
  local target_ref="${1:-}"
  local target_display="${target_ref:-<none>}"
  cat <<EOF_PLAN
This is a read-only setup plan for ${EXAMPLE_NAME}.
Nothing will be created or mutated.

Inputs:
- prefix: $(state_prefix)
- deploy namespace: ${DEPLOY_NAMESPACE}
- target: ${target_display}

Spaces that ./setup.sh will create:
- $(base_space)
- $(region_space)
- $(role_space)
- $(recipe_space)
- $(deploy_space)

Variant chain:
- $(base_space)/${BASE_UNIT}
- $(region_space)/${REGION_UNIT}
- $(role_space)/${ROLE_UNIT}
- $(recipe_space)/${RECIPE_UNIT}
- $(deploy_space)/${DEPLOY_UNIT}

Dependency stub:
- $(deploy_space)/${DEPLOY_STUB_UNIT}

Layer mutations:
- region: set backend REGION=${REGION_VALUE} and regional ingress host
- role: set ROLE=${ROLE_VALUE}, replicas=2, and LOG_LEVEL=info
- recipe: set CHAT_TITLE for the resolved recipe
- deployment: set namespace=${DEPLOY_NAMESPACE}, CLUSTER=${DEPLOY_NAMESPACE}, and deployment ingress host
- dependency stub: create postgres stub and set its namespace

Recipe manifest:
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}
- source template: ${RECIPE_BASE_TEMPLATE}
- rendered output: ${STATE_DIR}/recipe-us-staging.rendered.yaml

Core cub commands:
- cub space create
- cub unit create <unit> <file>
- cub unit create --upstream-space ... --upstream-unit ...
- cub function do set-env
- cub function do set-replicas
- cub function do set-string-path
- cub function do set-namespace
EOF_PLAN
  if [[ -n "${target_ref}" ]]; then
    cat <<EOF_PLAN
- cub unit set-target ${target_ref}
EOF_PLAN
  fi
}

show_setup_plan_json() {
  local target_ref="${1:-}"
  jq -n \
    --arg example "${EXAMPLE_NAME}" \
    --arg prefix "$(state_prefix)" \
    --arg namespace "${DEPLOY_NAMESPACE}" \
    --arg target "${target_ref}" \
    --arg source "${SOURCE_BACKEND_YAML}" \
    --arg stubSource "${POSTGRES_STUB_YAML}" \
    --arg baseSpace "$(base_space)" \
    --arg regionSpace "$(region_space)" \
    --arg roleSpace "$(role_space)" \
    --arg recipeSpace "$(recipe_space)" \
    --arg deploySpace "$(deploy_space)" \
    --arg baseUnit "${BASE_UNIT}" \
    --arg regionUnit "${REGION_UNIT}" \
    --arg roleUnit "${ROLE_UNIT}" \
    --arg recipeUnit "${RECIPE_UNIT}" \
    --arg deployUnit "${DEPLOY_UNIT}" \
    --arg stubUnit "${DEPLOY_STUB_UNIT}" \
    --arg manifestUnit "${RECIPE_MANIFEST_UNIT}" \
    --arg manifestTemplate "${RECIPE_BASE_TEMPLATE}" \
    --arg manifestRendered "${STATE_DIR}/recipe-us-staging.rendered.yaml" \
    '{
      example: $example,
      mode: "setup-plan",
      mutates: false,
      prefix: $prefix,
      namespace: $namespace,
      targetRef: (if $target == "" then null else $target end),
      spaces: [
        {stage: "base", space: $baseSpace},
        {stage: "region", space: $regionSpace},
        {stage: "role", space: $roleSpace},
        {stage: "recipe", space: $recipeSpace},
        {stage: "deployment", space: $deploySpace}
      ],
      components: [
        {
          component: "backend",
          source: $source,
          chain: [
            {stage: "base", space: $baseSpace, unit: $baseUnit},
            {stage: "region", space: $regionSpace, unit: $regionUnit},
            {stage: "role", space: $roleSpace, unit: $roleUnit},
            {stage: "recipe", space: $recipeSpace, unit: $recipeUnit},
            {stage: "deployment", space: $deploySpace, unit: $deployUnit}
          ],
          mutations: [
            {stage: "region", summary: "set backend REGION and regional ingress host"},
            {stage: "role", summary: "set ROLE, replicas, and LOG_LEVEL"},
            {stage: "recipe", summary: "set CHAT_TITLE"},
            {stage: "deployment", summary: "set namespace, CLUSTER, and deployment ingress host"}
          ]
        },
        {
          component: "postgres-stub",
          source: $stubSource,
          stub: true,
          chain: [
            {stage: "deployment", space: $deploySpace, unit: $stubUnit}
          ],
          mutations: [
            {stage: "deployment", summary: "set namespace"}
          ]
        }
      ],
      recipeManifest: {
        space: $recipeSpace,
        unit: $manifestUnit,
        template: $manifestTemplate,
        renderedOutput: $manifestRendered
      },
      commands: ([
        "cub space create",
        "cub unit create <unit> <file>",
        "cub unit create --upstream-space ... --upstream-unit ...",
        "cub function do set-env",
        "cub function do set-replicas",
        "cub function do set-string-path",
        "cub function do set-namespace"
      ] + (if $target == "" then [] else ["cub unit set-target"] end))
    }'
}

deploy_backend_hostname() {
  echo "backend.${DEPLOY_NAMESPACE}.demo.confighub.local"
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
  local recipe_space_url deploy_space_url recipe_unit_url deploy_unit_url
  recipe_space_url="$(gui_space_url "$(recipe_space)")"
  deploy_space_url="$(gui_space_url "$(deploy_space)")"
  recipe_unit_url="$(gui_unit_url "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}")"
  deploy_unit_url="$(gui_unit_url "$(deploy_space)" "${DEPLOY_UNIT}")"
  if [[ -n "${target_ref}" ]]; then
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

GUI:
- Recipe space: ${recipe_space_url}
- Deploy space: ${deploy_space_url}
- Recipe manifest: ${recipe_unit_url}
- Deployment unit: ${deploy_unit_url}

Logs:
- Setup log: $(current_log_path setup)
- Set-target log: $(current_log_path set-target)
- Verify log: $(current_log_path verify)
- Cleanup log: $(current_log_path cleanup)

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_IMAGE_TAG}
3. cub unit approve --space $(deploy_space) ${DEPLOY_UNIT} && cub unit apply --space $(deploy_space) ${DEPLOY_UNIT}
4. Review recipe manifest: cub unit get --space $(recipe_space) --data-only ${RECIPE_MANIFEST_UNIT}
EOF_SUMMARY
  else
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

GUI:
- Recipe space: ${recipe_space_url}
- Deploy space: ${deploy_space_url}
- Recipe manifest: ${recipe_unit_url}
- Deployment unit: ${deploy_unit_url}

Logs:
- Setup log: $(current_log_path setup)
- Set-target log: $(current_log_path set-target)
- Verify log: $(current_log_path verify)
- Cleanup log: $(current_log_path cleanup)

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_IMAGE_TAG}
3. ./set-target.sh <space/target>  # optional, enables apply + target bundle story
EOF_SUMMARY
  fi
}
