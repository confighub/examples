#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="${SCRIPT_DIR}/.state"
STATE_FILE="${STATE_DIR}/state.env"
RECIPE_BASE_TEMPLATE="${SCRIPT_DIR}/recipe.base.yaml"

EXAMPLE_NAME="global-app-layer-gpu-eks-h100-training"
CHAIN_LABEL="gpu-eks-h100-ubuntu-training-stack"
APP_NAME="gpu-platform"
COMPONENTS=(gpu-operator nvidia-device-plugin)
STAGES=(base platform accelerator os recipe deployment)

BASE_SPACE_SUFFIX="catalog-base"
PLATFORM_SPACE_SUFFIX="catalog-eks"
ACCELERATOR_SPACE_SUFFIX="catalog-h100"
OS_SPACE_SUFFIX="catalog-ubuntu"
RECIPE_SPACE_SUFFIX="recipe-eks-h100-ubuntu-training"
DEPLOY_SPACE_SUFFIX="deploy-cluster-a"

RECIPE_MANIFEST_UNIT="recipe-eks-h100-ubuntu-training-stack"
PLATFORM_VALUE="eks"
ACCELERATOR_VALUE="h100"
OS_VALUE="ubuntu"
INTENT_VALUE="training"
DEPLOY_NAMESPACE="${DEPLOY_NAMESPACE:-cluster-a}"
DEFAULT_GPU_OPERATOR_TAG="24.6.1"
DEFAULT_DEVICE_PLUGIN_TAG="v0.16.3"

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

base_space() { space_name "${BASE_SPACE_SUFFIX}"; }
platform_space() { space_name "${PLATFORM_SPACE_SUFFIX}"; }
accelerator_space() { space_name "${ACCELERATOR_SPACE_SUFFIX}"; }
os_space() { space_name "${OS_SPACE_SUFFIX}"; }
recipe_space() { space_name "${RECIPE_SPACE_SUFFIX}"; }
deploy_space() { space_name "${DEPLOY_SPACE_SUFFIX}"; }

space_for_stage() {
  local stage="$1"
  case "${stage}" in
    base) base_space ;;
    platform) platform_space ;;
    accelerator) accelerator_space ;;
    os) os_space ;;
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
    gpu-operator) echo "${SCRIPT_DIR}/gpu-operator.base.yaml" ;;
    nvidia-device-plugin) echo "${SCRIPT_DIR}/nvidia-device-plugin.base.yaml" ;;
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
    platform) echo "${component}-eks" ;;
    accelerator) echo "${component}-eks-h100" ;;
    os) echo "${component}-eks-h100-ubuntu" ;;
    recipe) echo "${component}-eks-h100-ubuntu-training" ;;
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
- $(platform_space)
- $(accelerator_space)
- $(os_space)
- $(recipe_space)
- $(deploy_space)

Components and variant chains:
- gpu-operator: $(base_space)/$(unit_name gpu-operator base) -> $(platform_space)/$(unit_name gpu-operator platform) -> $(accelerator_space)/$(unit_name gpu-operator accelerator) -> $(os_space)/$(unit_name gpu-operator os) -> $(recipe_space)/$(unit_name gpu-operator recipe) -> $(deploy_space)/$(unit_name gpu-operator deployment)
- nvidia-device-plugin: $(base_space)/$(unit_name nvidia-device-plugin base) -> $(platform_space)/$(unit_name nvidia-device-plugin platform) -> $(accelerator_space)/$(unit_name nvidia-device-plugin accelerator) -> $(os_space)/$(unit_name nvidia-device-plugin os) -> $(recipe_space)/$(unit_name nvidia-device-plugin recipe) -> $(deploy_space)/$(unit_name nvidia-device-plugin deployment)

Layer mutations:
- platform: set EKS-specific cloud and storage/plugin settings
- accelerator: set H100-specific accelerator and node selector settings
- os: set Ubuntu-specific OS and driver/plugin settings
- recipe: set training intent and validation/plugin profile
- deployment: set namespace=${DEPLOY_NAMESPACE} and CLUSTER=${DEPLOY_NAMESPACE}

Recipe manifest:
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}
- source template: ${RECIPE_BASE_TEMPLATE}
- rendered output: ${STATE_DIR}/recipe-eks-h100-ubuntu-training-stack.rendered.yaml

Core cub commands:
- cub space create
- cub unit create <unit> <file>
- cub unit create --upstream-space ... --upstream-unit ...
- cub function do set-env
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
    --arg gpuSource "$(source_yaml_for gpu-operator)" \
    --arg pluginSource "$(source_yaml_for nvidia-device-plugin)" \
    --arg baseSpace "$(base_space)" \
    --arg platformSpace "$(platform_space)" \
    --arg acceleratorSpace "$(accelerator_space)" \
    --arg osSpace "$(os_space)" \
    --arg recipeSpace "$(recipe_space)" \
    --arg deploySpace "$(deploy_space)" \
    --arg gpuBase "$(unit_name gpu-operator base)" \
    --arg gpuPlatform "$(unit_name gpu-operator platform)" \
    --arg gpuAccelerator "$(unit_name gpu-operator accelerator)" \
    --arg gpuOS "$(unit_name gpu-operator os)" \
    --arg gpuRecipe "$(unit_name gpu-operator recipe)" \
    --arg gpuDeploy "$(unit_name gpu-operator deployment)" \
    --arg pluginBase "$(unit_name nvidia-device-plugin base)" \
    --arg pluginPlatform "$(unit_name nvidia-device-plugin platform)" \
    --arg pluginAccelerator "$(unit_name nvidia-device-plugin accelerator)" \
    --arg pluginOS "$(unit_name nvidia-device-plugin os)" \
    --arg pluginRecipe "$(unit_name nvidia-device-plugin recipe)" \
    --arg pluginDeploy "$(unit_name nvidia-device-plugin deployment)" \
    --arg manifestUnit "${RECIPE_MANIFEST_UNIT}" \
    --arg manifestTemplate "${RECIPE_BASE_TEMPLATE}" \
    --arg manifestRendered "${STATE_DIR}/recipe-eks-h100-ubuntu-training-stack.rendered.yaml" \
    '{
      example: $example,
      mode: "setup-plan",
      mutates: false,
      prefix: $prefix,
      namespace: $namespace,
      targetRef: (if $target == "" then null else $target end),
      spaces: [
        {stage: "base", space: $baseSpace},
        {stage: "platform", space: $platformSpace},
        {stage: "accelerator", space: $acceleratorSpace},
        {stage: "os", space: $osSpace},
        {stage: "recipe", space: $recipeSpace},
        {stage: "deployment", space: $deploySpace}
      ],
      components: [
        {
          component: "gpu-operator",
          source: $gpuSource,
          chain: [
            {stage: "base", space: $baseSpace, unit: $gpuBase},
            {stage: "platform", space: $platformSpace, unit: $gpuPlatform},
            {stage: "accelerator", space: $acceleratorSpace, unit: $gpuAccelerator},
            {stage: "os", space: $osSpace, unit: $gpuOS},
            {stage: "recipe", space: $recipeSpace, unit: $gpuRecipe},
            {stage: "deployment", space: $deploySpace, unit: $gpuDeploy}
          ],
          mutations: [
            {stage: "platform", summary: "set cloud provider and storage class"},
            {stage: "accelerator", summary: "set accelerator and node selector"},
            {stage: "os", summary: "set OS family and driver branch"},
            {stage: "recipe", summary: "set workload intent and validation profile"},
            {stage: "deployment", summary: "set namespace and CLUSTER"}
          ]
        },
        {
          component: "nvidia-device-plugin",
          source: $pluginSource,
          chain: [
            {stage: "base", space: $baseSpace, unit: $pluginBase},
            {stage: "platform", space: $platformSpace, unit: $pluginPlatform},
            {stage: "accelerator", space: $acceleratorSpace, unit: $pluginAccelerator},
            {stage: "os", space: $osSpace, unit: $pluginOS},
            {stage: "recipe", space: $recipeSpace, unit: $pluginRecipe},
            {stage: "deployment", space: $deploySpace, unit: $pluginDeploy}
          ],
          mutations: [
            {stage: "platform", summary: "set cloud provider and EKS plugin config"},
            {stage: "accelerator", summary: "set accelerator and node selector"},
            {stage: "os", summary: "set OS family and Ubuntu plugin config"},
            {stage: "recipe", summary: "set workload intent and training plugin config"},
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
        "cub function do set-namespace"
      ] + (if $target == "" then [] else ["cub unit set-target"] end))
    }'
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
  local rendered_manifest="${STATE_DIR}/recipe-eks-h100-ubuntu-training-stack.rendered.yaml"
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

apply_platform_mutations() {
  local component="$1"
  case "${component}" in
    gpu-operator)
      cub function do set-env gpu-operator "CLOUD_PROVIDER=${PLATFORM_VALUE}" --space "$(platform_space)" --unit "$(unit_name "${component}" platform)"
      cub function do set-env gpu-operator "STORAGE_CLASS=gp3" --space "$(platform_space)" --unit "$(unit_name "${component}" platform)"
      ;;
    nvidia-device-plugin)
      cub function do set-env nvidia-device-plugin "CLOUD_PROVIDER=${PLATFORM_VALUE}" --space "$(platform_space)" --unit "$(unit_name "${component}" platform)"
      cub function do set-env nvidia-device-plugin "PLUGIN_CONFIG=eks-gp3" --space "$(platform_space)" --unit "$(unit_name "${component}" platform)"
      ;;
  esac
}

apply_accelerator_mutations() {
  local component="$1"
  case "${component}" in
    gpu-operator)
      cub function do set-env gpu-operator "ACCELERATOR=${ACCELERATOR_VALUE}" --space "$(accelerator_space)" --unit "$(unit_name "${component}" accelerator)"
      cub function do set-env gpu-operator "NODE_SELECTOR=nvidia-h100" --space "$(accelerator_space)" --unit "$(unit_name "${component}" accelerator)"
      ;;
    nvidia-device-plugin)
      cub function do set-env nvidia-device-plugin "ACCELERATOR=${ACCELERATOR_VALUE}" --space "$(accelerator_space)" --unit "$(unit_name "${component}" accelerator)"
      cub function do set-env nvidia-device-plugin "NODE_SELECTOR=nvidia-h100" --space "$(accelerator_space)" --unit "$(unit_name "${component}" accelerator)"
      ;;
  esac
}

apply_os_mutations() {
  local component="$1"
  case "${component}" in
    gpu-operator)
      cub function do set-env gpu-operator "OS_FAMILY=${OS_VALUE}" --space "$(os_space)" --unit "$(unit_name "${component}" os)"
      cub function do set-env gpu-operator "DRIVER_BRANCH=550-ubuntu22.04" --space "$(os_space)" --unit "$(unit_name "${component}" os)"
      ;;
    nvidia-device-plugin)
      cub function do set-env nvidia-device-plugin "OS_FAMILY=${OS_VALUE}" --space "$(os_space)" --unit "$(unit_name "${component}" os)"
      cub function do set-env nvidia-device-plugin "PLUGIN_CONFIG=ubuntu-h100" --space "$(os_space)" --unit "$(unit_name "${component}" os)"
      ;;
  esac
}

apply_recipe_mutations() {
  local component="$1"
  case "${component}" in
    gpu-operator)
      cub function do set-env gpu-operator "WORKLOAD_INTENT=${INTENT_VALUE}" --space "$(recipe_space)" --unit "$(unit_name "${component}" recipe)"
      cub function do set-env gpu-operator "VALIDATION_PROFILE=training-smoke" --space "$(recipe_space)" --unit "$(unit_name "${component}" recipe)"
      ;;
    nvidia-device-plugin)
      cub function do set-env nvidia-device-plugin "WORKLOAD_INTENT=${INTENT_VALUE}" --space "$(recipe_space)" --unit "$(unit_name "${component}" recipe)"
      cub function do set-env nvidia-device-plugin "PLUGIN_CONFIG=training-smoke" --space "$(recipe_space)" --unit "$(unit_name "${component}" recipe)"
      ;;
  esac
}

apply_deploy_mutations() {
  local component="$1"
  cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "$(unit_name "${component}" deployment)"
  case "${component}" in
    gpu-operator)
      cub function do set-env gpu-operator "CLUSTER=${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "$(unit_name "${component}" deployment)"
      ;;
    nvidia-device-plugin)
      cub function do set-env nvidia-device-plugin "CLUSTER=${DEPLOY_NAMESPACE}" --space "$(deploy_space)" --unit "$(unit_name "${component}" deployment)"
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
Created GPU layered chain with prefix: $(state_prefix)

Spaces:
- $(base_space)
- $(platform_space)
- $(accelerator_space)
- $(os_space)
- $(recipe_space)
- $(deploy_space)

Units:
- $(base_space)/$(unit_name gpu-operator base)
- $(platform_space)/$(unit_name gpu-operator platform)
- $(accelerator_space)/$(unit_name gpu-operator accelerator)
- $(os_space)/$(unit_name gpu-operator os)
- $(recipe_space)/$(unit_name gpu-operator recipe)
- $(deploy_space)/$(unit_name gpu-operator deployment)
- $(base_space)/$(unit_name nvidia-device-plugin base)
- $(platform_space)/$(unit_name nvidia-device-plugin platform)
- $(accelerator_space)/$(unit_name nvidia-device-plugin accelerator)
- $(os_space)/$(unit_name nvidia-device-plugin os)
- $(recipe_space)/$(unit_name nvidia-device-plugin recipe)
- $(deploy_space)/$(unit_name nvidia-device-plugin deployment)
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_GPU_OPERATOR_TAG} ${DEFAULT_DEVICE_PLUGIN_TAG}
3. cub unit approve --space $(deploy_space) $(unit_name gpu-operator deployment) && cub unit approve --space $(deploy_space) $(unit_name nvidia-device-plugin deployment)
4. cub unit apply --space $(deploy_space) $(unit_name gpu-operator deployment) && cub unit apply --space $(deploy_space) $(unit_name nvidia-device-plugin deployment)
5. Review recipe manifest: cub unit get --space $(recipe_space) --data-only ${RECIPE_MANIFEST_UNIT}
EOF_SUMMARY
  else
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
- $(base_space)/$(unit_name gpu-operator base)
- $(platform_space)/$(unit_name gpu-operator platform)
- $(accelerator_space)/$(unit_name gpu-operator accelerator)
- $(os_space)/$(unit_name gpu-operator os)
- $(recipe_space)/$(unit_name gpu-operator recipe)
- $(deploy_space)/$(unit_name gpu-operator deployment)
- $(base_space)/$(unit_name nvidia-device-plugin base)
- $(platform_space)/$(unit_name nvidia-device-plugin platform)
- $(accelerator_space)/$(unit_name nvidia-device-plugin accelerator)
- $(os_space)/$(unit_name nvidia-device-plugin os)
- $(recipe_space)/$(unit_name nvidia-device-plugin recipe)
- $(deploy_space)/$(unit_name nvidia-device-plugin deployment)
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_GPU_OPERATOR_TAG} ${DEFAULT_DEVICE_PLUGIN_TAG}
3. ./set-target.sh <space/target>  # optional, enables apply + target bundle story
EOF_SUMMARY
  fi
}
