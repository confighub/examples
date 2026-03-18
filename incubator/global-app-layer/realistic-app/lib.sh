#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="${SCRIPT_DIR}/.state"
STATE_FILE="${STATE_DIR}/state.env"
RECIPE_BASE_TEMPLATE="${SCRIPT_DIR}/recipe.base.yaml"

EXAMPLE_NAME="global-app-layer-realistic-app"
CHAIN_LABEL="global-app-us-staging-realistic"
APP_NAME="global-app"
COMPONENTS=(backend frontend postgres)
STAGES=(base region role recipe deployment)

BASE_SPACE_SUFFIX="catalog-base"
REGION_SPACE_SUFFIX="catalog-us"
ROLE_SPACE_SUFFIX="catalog-us-staging"
RECIPE_SPACE_SUFFIX="recipe-us-staging"
DEPLOY_SPACE_SUFFIX="deploy-cluster-a"

RECIPE_MANIFEST_UNIT="recipe-us-staging-realistic-app"
REGION_VALUE="us"
ROLE_VALUE="staging"
DEPLOY_NAMESPACE="${DEPLOY_NAMESPACE:-cluster-a}"
DEFAULT_BACKEND_TAG="1.1.8"
DEFAULT_FRONTEND_TAG="1.1.8"
DEFAULT_POSTGRES_TAG="16.1"

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

space_for_stage() {
  local stage="$1"
  case "${stage}" in
    base) base_space ;;
    region) region_space ;;
    role) role_space ;;
    recipe) recipe_space ;;
    deployment) deploy_space ;;
    *)
      echo "Unknown stage: ${stage}" >&2
      exit 1
      ;;
  esac
}

source_yaml_for() {
  local component="$1"
  case "${component}" in
    backend) echo "${SCRIPT_DIR}/../../../global-app/baseconfig/backend.yaml" ;;
    frontend) echo "${SCRIPT_DIR}/../../../global-app/baseconfig/frontend.yaml" ;;
    postgres) echo "${SCRIPT_DIR}/../../../global-app/baseconfig/postgres.yaml" ;;
    *)
      echo "Unknown component: ${component}" >&2
      exit 1
      ;;
  esac
}

unit_name() {
  local component="$1"
  local stage="$2"
  case "${stage}" in
    base) echo "${component}-base" ;;
    region) echo "${component}-us" ;;
    role) echo "${component}-us-staging" ;;
    recipe) echo "${component}-recipe-us-staging" ;;
    deployment) echo "${component}-cluster-a" ;;
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

label_args() {
  local layer="$1"
  local component="$2"
  shift 2
  printf '%s\n' \
    --label "ExampleName=${EXAMPLE_NAME}" \
    --label "ExampleChain=$(state_prefix)" \
    --label "Recipe=${CHAIN_LABEL}" \
    --label "App=${APP_NAME}" \
    --label "Component=${component}" \
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
    --label "App=${APP_NAME}" \
    --label "LayerKind=${layer_kind}" \
    "$@"
  local component
  for component in "${COMPONENTS[@]}"; do
    printf '%s\n' --label "Component=${component}"
  done
}

assert_contains() {
  local file_path="$1"
  local pattern="$2"
  # Try exact fixed-string match first; if that fails and the pattern
  # contains "value: ", retry with optional YAML quotes around the value
  # to tolerate cub's inconsistent quoting of env-var values.
  if grep -q --fixed-strings "${pattern}" "${file_path}"; then
    return 0
  fi
  if [[ "${pattern}" == *'value: '* ]]; then
    local prefix="${pattern%%value: *}value: "
    local val="${pattern#*value: }"
    # try with double-quotes around the value
    if grep -q --fixed-strings "${prefix}\"${val}\"" "${file_path}"; then
      return 0
    fi
    # try without quotes (in case the assertion had quotes but the file doesn't)
    local unquoted_val="${val#\"}"
    unquoted_val="${unquoted_val%\"}"
    if grep -q --fixed-strings "${prefix}${unquoted_val}" "${file_path}"; then
      return 0
    fi
  fi
  echo "Expected pattern not found in ${file_path}: ${pattern}" >&2
  exit 1
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

Components and variant chains:
- backend: $(base_space)/$(unit_name backend base) -> $(region_space)/$(unit_name backend region) -> $(role_space)/$(unit_name backend role) -> $(recipe_space)/$(unit_name backend recipe) -> $(deploy_space)/$(unit_name backend deployment)
- frontend: $(base_space)/$(unit_name frontend base) -> $(region_space)/$(unit_name frontend region) -> $(role_space)/$(unit_name frontend role) -> $(recipe_space)/$(unit_name frontend recipe) -> $(deploy_space)/$(unit_name frontend deployment)
- postgres: $(base_space)/$(unit_name postgres base) -> $(region_space)/$(unit_name postgres region) -> $(role_space)/$(unit_name postgres role) -> $(recipe_space)/$(unit_name postgres recipe) -> $(deploy_space)/$(unit_name postgres deployment)

Layer mutations:
- backend region: set REGION=${REGION_VALUE} and regional ingress host
- backend role: set replicas=2 and ROLE=${ROLE_VALUE}
- backend recipe: set DATABASE_URL and CHAT_TITLE
- backend deployment: set namespace, CLUSTER=${DEPLOY_NAMESPACE}, and deployment ingress host
- frontend region: set regional ingress host
- frontend role: set replicas=2 and PUBLIC_ENV=${ROLE_VALUE}
- frontend recipe: set RELEASE_CHANNEL
- frontend deployment: set namespace, CLUSTER=${DEPLOY_NAMESPACE}, and deployment ingress host
- postgres region: set REGION=US
- postgres role: set PVC size and ROLE=${ROLE_VALUE}
- postgres recipe: set POSTGRES_DB
- postgres deployment: set namespace and CLUSTER=${DEPLOY_NAMESPACE}

Recipe manifest:
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}
- source template: ${RECIPE_BASE_TEMPLATE}
- rendered output: ${STATE_DIR}/recipe-us-staging-realistic-app.rendered.yaml

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
    --arg backendSource "$(source_yaml_for backend)" \
    --arg frontendSource "$(source_yaml_for frontend)" \
    --arg postgresSource "$(source_yaml_for postgres)" \
    --arg baseSpace "$(base_space)" \
    --arg regionSpace "$(region_space)" \
    --arg roleSpace "$(role_space)" \
    --arg recipeSpace "$(recipe_space)" \
    --arg deploySpace "$(deploy_space)" \
    --arg backendBase "$(unit_name backend base)" \
    --arg backendRegion "$(unit_name backend region)" \
    --arg backendRole "$(unit_name backend role)" \
    --arg backendRecipe "$(unit_name backend recipe)" \
    --arg backendDeploy "$(unit_name backend deployment)" \
    --arg frontendBase "$(unit_name frontend base)" \
    --arg frontendRegion "$(unit_name frontend region)" \
    --arg frontendRole "$(unit_name frontend role)" \
    --arg frontendRecipe "$(unit_name frontend recipe)" \
    --arg frontendDeploy "$(unit_name frontend deployment)" \
    --arg postgresBase "$(unit_name postgres base)" \
    --arg postgresRegion "$(unit_name postgres region)" \
    --arg postgresRole "$(unit_name postgres role)" \
    --arg postgresRecipe "$(unit_name postgres recipe)" \
    --arg postgresDeploy "$(unit_name postgres deployment)" \
    --arg manifestUnit "${RECIPE_MANIFEST_UNIT}" \
    --arg manifestTemplate "${RECIPE_BASE_TEMPLATE}" \
    --arg manifestRendered "${STATE_DIR}/recipe-us-staging-realistic-app.rendered.yaml" \
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
          source: $backendSource,
          chain: [
            {stage: "base", space: $baseSpace, unit: $backendBase},
            {stage: "region", space: $regionSpace, unit: $backendRegion},
            {stage: "role", space: $roleSpace, unit: $backendRole},
            {stage: "recipe", space: $recipeSpace, unit: $backendRecipe},
            {stage: "deployment", space: $deploySpace, unit: $backendDeploy}
          ],
          mutations: [
            {stage: "region", summary: "set REGION and regional ingress host"},
            {stage: "role", summary: "set replicas and ROLE"},
            {stage: "recipe", summary: "set DATABASE_URL and CHAT_TITLE"},
            {stage: "deployment", summary: "set namespace, CLUSTER, and deployment ingress host"}
          ]
        },
        {
          component: "frontend",
          source: $frontendSource,
          chain: [
            {stage: "base", space: $baseSpace, unit: $frontendBase},
            {stage: "region", space: $regionSpace, unit: $frontendRegion},
            {stage: "role", space: $roleSpace, unit: $frontendRole},
            {stage: "recipe", space: $recipeSpace, unit: $frontendRecipe},
            {stage: "deployment", space: $deploySpace, unit: $frontendDeploy}
          ],
          mutations: [
            {stage: "region", summary: "set regional ingress host"},
            {stage: "role", summary: "set replicas and PUBLIC_ENV"},
            {stage: "recipe", summary: "set RELEASE_CHANNEL"},
            {stage: "deployment", summary: "set namespace, CLUSTER, and deployment ingress host"}
          ]
        },
        {
          component: "postgres",
          source: $postgresSource,
          chain: [
            {stage: "base", space: $baseSpace, unit: $postgresBase},
            {stage: "region", space: $regionSpace, unit: $postgresRegion},
            {stage: "role", space: $roleSpace, unit: $postgresRole},
            {stage: "recipe", space: $recipeSpace, unit: $postgresRecipe},
            {stage: "deployment", space: $deploySpace, unit: $postgresDeploy}
          ],
          mutations: [
            {stage: "region", summary: "set REGION"},
            {stage: "role", summary: "set PVC size and ROLE"},
            {stage: "recipe", summary: "set POSTGRES_DB"},
            {stage: "deployment", summary: "set namespace and CLUSTER"}
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

deploy_hostname() {
  local component="$1"
  case "${component}" in
    backend) echo "backend.${DEPLOY_NAMESPACE}.demo.confighub.local" ;;
    frontend) echo "frontend.${DEPLOY_NAMESPACE}.demo.confighub.local" ;;
    *) return 1 ;;
  esac
}

render_recipe_manifest() {
  local output_path="$1"
  local target_ref="$2"
  local effective_target_ref="${target_ref:-unset}"
  local sed_args=()
  local component stage space unit revision hash

  sed_args+=(
    -e "s|confighubplaceholder-chain-name|${CHAIN_LABEL}|g"
    -e "s|confighubplaceholder-example-name|${EXAMPLE_NAME}|g"
    -e "s|confighubplaceholder-chain-prefix|$(state_prefix)|g"
    -e "s|confighubplaceholder-app-name|${APP_NAME}|g"
    -e "s|confighubplaceholder-target-ref|${effective_target_ref}|g"
    -e "s|confighubplaceholder-bundle-hint|$(bundle_hint_from_target_ref "${target_ref}")|g"
  )

  for component in "${COMPONENTS[@]}"; do
    for stage in "${STAGES[@]}"; do
      space="$(space_for_stage "${stage}")"
      unit="$(unit_name "${component}" "${stage}")"
      revision="$(get_unit_field "${space}" "${unit}" HeadRevisionNum)"
      hash="$(get_unit_field "${space}" "${unit}" DataHash || true)"
      sed_args+=(
        -e "s|confighubplaceholder-${component}-${stage}-space|${space}|g"
        -e "s|confighubplaceholder-${component}-${stage}-unit|${unit}|g"
        -e "s|confighubplaceholder-${component}-${stage}-revision|${revision}|g"
        -e "s|confighubplaceholder-${component}-${stage}-hash|${hash}|g"
      )
    done
  done

  sed "${sed_args[@]}" "${RECIPE_BASE_TEMPLATE}" >"${output_path}"
}

refresh_recipe_manifest_unit() {
  local target_ref="$1"
  local rendered_manifest="${STATE_DIR}/recipe-us-staging-realistic-app.rendered.yaml"
  ensure_state_dir
  render_recipe_manifest "${rendered_manifest}" "${target_ref}"

  if unit_exists "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}"; then
    cub unit update --space "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}" "${rendered_manifest}"
  else
    _mapfile manifest_labels < <(label_args recipe-manifest app)
    cub unit create --space "$(recipe_space)" -t AppConfig/YAML \
      "${RECIPE_MANIFEST_UNIT}" "${rendered_manifest}" "${manifest_labels[@]}"
  fi
}

apply_region_mutations() {
  local component="$1"
  local space unit
  space="$(region_space)"
  unit="$(unit_name "${component}" region)"

  case "${component}" in
    backend)
      cub function do set-env backend "REGION=${REGION_VALUE}" --space "${space}" --unit "${unit}"
      cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host backend.us.demo.confighub.local --space "${space}" --unit "${unit}"
      ;;
    frontend)
      cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host frontend.us.demo.confighub.local --space "${space}" --unit "${unit}"
      ;;
    postgres)
      cub function do set-env postgres "REGION=US" --space "${space}" --unit "${unit}"
      ;;
  esac
}

apply_role_mutations() {
  local component="$1"
  local space unit
  space="$(role_space)"
  unit="$(unit_name "${component}" role)"

  case "${component}" in
    backend)
      cub function do set-replicas 2 --space "${space}" --unit "${unit}"
      cub function do set-env backend "ROLE=${ROLE_VALUE}" --space "${space}" --unit "${unit}"
      ;;
    frontend)
      cub function do set-replicas 2 --space "${space}" --unit "${unit}"
      cub function do set-env frontend "PUBLIC_ENV=${ROLE_VALUE}" --space "${space}" --unit "${unit}"
      ;;
    postgres)
      cub function do set-string-path apps/v1/StatefulSet spec.volumeClaimTemplates.0.spec.resources.requests.storage 10Gi --space "${space}" --unit "${unit}"
      cub function do set-env postgres "ROLE=${ROLE_VALUE}" --space "${space}" --unit "${unit}"
      ;;
  esac
}

apply_recipe_mutations() {
  local component="$1"
  local space unit
  space="$(recipe_space)"
  unit="$(unit_name "${component}" recipe)"

  case "${component}" in
    backend)
      cub function do set-env backend "DATABASE_URL=postgres://admin:password@postgres:5432/chatdb_us_staging" --space "${space}" --unit "${unit}"
      cub function do set-env backend "CHAT_TITLE=Cubby Chat US Staging" --space "${space}" --unit "${unit}"
      ;;
    frontend)
      cub function do set-env frontend "RELEASE_CHANNEL=us-staging-recipe" --space "${space}" --unit "${unit}"
      ;;
    postgres)
      cub function do set-env postgres "POSTGRES_DB=chatdb_us_staging" --space "${space}" --unit "${unit}"
      ;;
  esac
}

apply_deploy_mutations() {
  local component="$1"
  local space unit
  space="$(deploy_space)"
  unit="$(unit_name "${component}" deployment)"

  cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "${space}" --unit "${unit}"

  case "${component}" in
    backend)
      cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host "$(deploy_hostname backend)" --space "${space}" --unit "${unit}"
      cub function do set-env backend "CLUSTER=${DEPLOY_NAMESPACE}" --space "${space}" --unit "${unit}"
      ;;
    frontend)
      cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host "$(deploy_hostname frontend)" --space "${space}" --unit "${unit}"
      cub function do set-env frontend "CLUSTER=${DEPLOY_NAMESPACE}" --space "${space}" --unit "${unit}"
      ;;
    postgres)
      cub function do set-env postgres "CLUSTER=${DEPLOY_NAMESPACE}" --space "${space}" --unit "${unit}"
      ;;
  esac
}

set_target_for_deploy_units() {
  local target_ref="$1"
  local component
  for component in "${COMPONENTS[@]}"; do
    cub unit set-target "${target_ref}" --space "$(deploy_space)" --unit "$(unit_name "${component}" deployment)"
  done
}

show_summary() {
  local target_ref="$1"
  if [[ -n "${target_ref}" ]]; then
    cat <<EOF_SUMMARY
Created realistic global-app layered chain with prefix: $(state_prefix)

Spaces:
- $(base_space)
- $(region_space)
- $(role_space)
- $(recipe_space)
- $(deploy_space)

Units:
- $(base_space)/$(unit_name backend base)
- $(region_space)/$(unit_name backend region)
- $(role_space)/$(unit_name backend role)
- $(recipe_space)/$(unit_name backend recipe)
- $(deploy_space)/$(unit_name backend deployment)
- $(base_space)/$(unit_name frontend base)
- $(region_space)/$(unit_name frontend region)
- $(role_space)/$(unit_name frontend role)
- $(recipe_space)/$(unit_name frontend recipe)
- $(deploy_space)/$(unit_name frontend deployment)
- $(base_space)/$(unit_name postgres base)
- $(region_space)/$(unit_name postgres region)
- $(role_space)/$(unit_name postgres role)
- $(recipe_space)/$(unit_name postgres recipe)
- $(deploy_space)/$(unit_name postgres deployment)
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_BACKEND_TAG} ${DEFAULT_FRONTEND_TAG} ${DEFAULT_POSTGRES_TAG}
3. cub unit approve --space $(deploy_space) $(unit_name backend deployment) && cub unit approve --space $(deploy_space) $(unit_name frontend deployment) && cub unit approve --space $(deploy_space) $(unit_name postgres deployment)
4. cub unit apply --space $(deploy_space) $(unit_name backend deployment) && cub unit apply --space $(deploy_space) $(unit_name frontend deployment) && cub unit apply --space $(deploy_space) $(unit_name postgres deployment)
5. Review recipe manifest: cub unit get --space $(recipe_space) --data-only ${RECIPE_MANIFEST_UNIT}
EOF_SUMMARY
  else
    cat <<EOF_SUMMARY
Created realistic global-app layered chain with prefix: $(state_prefix)

Spaces:
- $(base_space)
- $(region_space)
- $(role_space)
- $(recipe_space)
- $(deploy_space)

Units:
- $(base_space)/$(unit_name backend base)
- $(region_space)/$(unit_name backend region)
- $(role_space)/$(unit_name backend role)
- $(recipe_space)/$(unit_name backend recipe)
- $(deploy_space)/$(unit_name backend deployment)
- $(base_space)/$(unit_name frontend base)
- $(region_space)/$(unit_name frontend region)
- $(role_space)/$(unit_name frontend role)
- $(recipe_space)/$(unit_name frontend recipe)
- $(deploy_space)/$(unit_name frontend deployment)
- $(base_space)/$(unit_name postgres base)
- $(region_space)/$(unit_name postgres region)
- $(role_space)/$(unit_name postgres role)
- $(recipe_space)/$(unit_name postgres recipe)
- $(deploy_space)/$(unit_name postgres deployment)
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_BACKEND_TAG} ${DEFAULT_FRONTEND_TAG} ${DEFAULT_POSTGRES_TAG}
3. ./set-target.sh <space/target>  # optional, enables apply + target bundle story
EOF_SUMMARY
  fi
}
