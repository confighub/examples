#!/usr/bin/env bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
STATE_DIR="${SCRIPT_DIR}/.state"
STATE_FILE="${STATE_DIR}/state.env"
LOG_DIR="${SCRIPT_DIR}/.logs"
RECIPE_BASE_TEMPLATE="${SCRIPT_DIR}/recipe.base.yaml"

EXAMPLE_NAME="global-app-layer-enterprise-rag-blueprint"
CHAIN_LABEL="enterprise-rag-blueprint-stack"
APP_NAME="enterprise-rag"
COMPONENTS=(rag-server nim-llm nim-embedding vector-db)
CHAIN_STAGES=(base platform accelerator profile recipe)
DEPLOY_VARIANTS=(direct flux argo)

BASE_SPACE_SUFFIX="catalog-base"
PLATFORM_SPACE_SUFFIX="catalog-kgpu"
ACCELERATOR_SPACE_SUFFIX="catalog-h100"
PROFILE_SPACE_SUFFIX="catalog-medium"
RECIPE_SPACE_SUFFIX="recipe-enterprise-rag"
DEPLOY_SPACE_SUFFIX="deploy-tenant-acme"
DEPLOY_FLUX_SPACE_SUFFIX="deploy-tenant-acme-flux"
DEPLOY_ARGO_SPACE_SUFFIX="deploy-tenant-acme-argo"

RECIPE_MANIFEST_UNIT="recipe-enterprise-rag-stack"
PLATFORM_VALUE="kgpu"
ACCELERATOR_VALUE="h100"
PROFILE_VALUE="medium"
RECIPE_VALUE="enterprise-rag"
DEPLOY_NAMESPACE="${DEPLOY_NAMESPACE:-tenant-acme}"
DEPLOY_REGION="${DEPLOY_REGION:-us-east}"

# STACK selector: stub | ollama | nim
# Default: stub (works on any cluster). On macOS, the README documents Ollama as a runtime path.
STACK="${STACK:-stub}"

# Default model versions per STACK. Bumped via upgrade-chain.sh.
DEFAULT_LLM_MODEL_NAME="${DEFAULT_LLM_MODEL_NAME:-llama3.2:3b}"
DEFAULT_LLM_MODEL_TAG="${DEFAULT_LLM_MODEL_TAG:-1.0.0}"
DEFAULT_EMBED_MODEL_NAME="${DEFAULT_EMBED_MODEL_NAME:-nomic-embed-text}"
DEFAULT_EMBED_DIM="${DEFAULT_EMBED_DIM:-768}"

GPU_USER_components_rag_server=false
GPU_USER_components_nim_llm=true
GPU_USER_components_nim_embedding=true
GPU_USER_components_vector_db=false

is_gpu_user() {
  local component="$1"
  case "${component}" in
    nim-llm|nim-embedding) echo true ;;
    *) echo false ;;
  esac
}

component_role() {
  local component="$1"
  case "${component}" in
    rag-server) echo "orchestration" ;;
    nim-llm) echo "answer-llm" ;;
    nim-embedding) echo "embedder" ;;
    vector-db) echo "vector-store" ;;
    *) echo "unknown" ;;
  esac
}

# Image refs per STACK x component. The PROFILE layer is what actually swaps these in.
image_ref_for() {
  local component="$1"
  local stack="${2:-${STACK}}"
  case "${stack}-${component}" in
    stub-rag-server) echo "python:3.12-slim" ;;
    stub-nim-llm) echo "nginx:1.27-alpine" ;;
    stub-nim-embedding) echo "nginx:1.27-alpine" ;;
    stub-vector-db) echo "busybox:1.37" ;;
    ollama-rag-server) echo "python:3.12-slim" ;;
    ollama-nim-llm) echo "nginx:1.27-alpine" ;; # placeholder; rag-server bypasses to host Ollama
    ollama-nim-embedding) echo "nginx:1.27-alpine" ;; # placeholder; rag-server uses host Ollama for embeddings
    ollama-vector-db) echo "qdrant/qdrant:v1.11.0" ;;
    nim-rag-server) echo "python:3.12-slim" ;;
    nim-nim-llm) echo "nvcr.io/nim/meta/${DEFAULT_LLM_MODEL_NAME}:${DEFAULT_LLM_MODEL_TAG}" ;;
    nim-nim-embedding) echo "nvcr.io/nim/nvidia/${DEFAULT_EMBED_MODEL_NAME}:${DEFAULT_LLM_MODEL_TAG}" ;;
    nim-vector-db) echo "milvusdb/milvus:v2.4.5" ;;
    *) echo "" ;;
  esac
}

# Where rag-server should reach the LLM and embedding services for a given STACK.
# This is the deployment-layer override that proves Story 3 (fleet variants).
llm_host_for() {
  local stack="${1:-${STACK}}"
  case "${stack}" in
    ollama) echo "host.docker.internal:11434" ;;
    nim) echo "nim-llm.${DEPLOY_NAMESPACE}.svc.cluster.local:8000" ;;
    *) echo "nim-llm.${DEPLOY_NAMESPACE}.svc.cluster.local:8000" ;;
  esac
}

embedding_host_for() {
  local stack="${1:-${STACK}}"
  case "${stack}" in
    ollama) echo "host.docker.internal:11434" ;;
    nim) echo "nim-embedding.${DEPLOY_NAMESPACE}.svc.cluster.local:8001" ;;
    *) echo "nim-embedding.${DEPLOY_NAMESPACE}.svc.cluster.local:8001" ;;
  esac
}

vector_db_host_for() {
  local stack="${1:-${STACK}}"
  case "${stack}" in
    ollama) echo "vector-db.${DEPLOY_NAMESPACE}.svc.cluster.local:6333" ;;
    nim) echo "vector-db.${DEPLOY_NAMESPACE}.svc.cluster.local:19530" ;;
    *) echo "vector-db.${DEPLOY_NAMESPACE}.svc.cluster.local:6333" ;;
  esac
}

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

setup_usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh [prefix] [target-ref ...]
  ./setup.sh --explain [prefix] [target-ref ...]
  ./setup.sh --explain-json [prefix] [target-ref ...]

Modes:
  default         Create the full chain in ConfigHub (mutating)
  --explain       Show a read-only plan of what setup would create
  --explain-json  Emit the same plan as machine-readable JSON

Environment:
  STACK           stub | ollama | nim   (default: stub)
                  Selects which image refs and host endpoints the recipe uses.
                  See README.md "Runtime story" for details.
  DEPLOY_NAMESPACE  Override tenant namespace (default: tenant-acme).
  DEPLOY_REGION   Override region label (default: us-east).
EOF_USAGE
}

verify_usage() {
  cat <<'EOF_USAGE'
Usage:
  ./verify.sh
  ./verify.sh --json

Modes:
  default   Run verification and print human-readable progress
  --json    Emit machine-readable verification output
EOF_USAGE
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
  : "${DIRECT_TARGET_REF:=${TARGET_REF:-}}"
  : "${FLUX_TARGET_REF:=}"
  : "${ARGO_TARGET_REF:=}"
  : "${STACK:=stub}"
}

save_state() {
  local prefix="$1"
  local direct_target_ref="${2:-}"
  local flux_target_ref="${3:-}"
  local argo_target_ref="${4:-}"

  ensure_state_dir
  printf 'PREFIX=%q\nTARGET_REF=%q\nDIRECT_TARGET_REF=%q\nFLUX_TARGET_REF=%q\nARGO_TARGET_REF=%q\nDEPLOY_NAMESPACE=%q\nDEPLOY_REGION=%q\nSTACK=%q\n' \
    "${prefix}" "${direct_target_ref}" "${direct_target_ref}" "${flux_target_ref}" "${argo_target_ref}" "${DEPLOY_NAMESPACE}" "${DEPLOY_REGION}" "${STACK}" >"${STATE_FILE}"
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
profile_space() { space_name "${PROFILE_SPACE_SUFFIX}"; }
recipe_space() { space_name "${RECIPE_SPACE_SUFFIX}"; }
deploy_space() { space_name "${DEPLOY_SPACE_SUFFIX}"; }
flux_deploy_space() { space_name "${DEPLOY_FLUX_SPACE_SUFFIX}"; }
argo_deploy_space() { space_name "${DEPLOY_ARGO_SPACE_SUFFIX}"; }

space_for_stage() {
  local stage="$1"
  case "${stage}" in
    base) base_space ;;
    platform) platform_space ;;
    accelerator) accelerator_space ;;
    profile) profile_space ;;
    recipe) recipe_space ;;
    *)
      echo "Unknown stage: ${stage}" >&2
      exit 1
      ;;
  esac
}

deploy_space_for_variant() {
  local variant="$1"
  case "${variant}" in
    direct) deploy_space ;;
    flux) flux_deploy_space ;;
    argo) argo_deploy_space ;;
    *)
      echo "Unknown deployment variant: ${variant}" >&2
      exit 1
      ;;
  esac
}

source_yaml_for() {
  local component="$1"
  case "${component}" in
    rag-server) echo "${SCRIPT_DIR}/rag-server.base.yaml" ;;
    nim-llm) echo "${SCRIPT_DIR}/nim-llm.base.yaml" ;;
    nim-embedding) echo "${SCRIPT_DIR}/nim-embedding.base.yaml" ;;
    vector-db) echo "${SCRIPT_DIR}/vector-db.base.yaml" ;;
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
    platform) echo "${component}-${PLATFORM_VALUE}" ;;
    accelerator) echo "${component}-${PLATFORM_VALUE}-${ACCELERATOR_VALUE}" ;;
    profile) echo "${component}-${PLATFORM_VALUE}-${ACCELERATOR_VALUE}-${PROFILE_VALUE}" ;;
    recipe) echo "${component}-${PLATFORM_VALUE}-${ACCELERATOR_VALUE}-${PROFILE_VALUE}-${RECIPE_VALUE}" ;;
    deployment) echo "${component}-${DEPLOY_NAMESPACE}" ;;
    *)
      echo "Unknown stage: ${stage}" >&2
      exit 1
      ;;
  esac
}

deployment_unit_name() {
  local component="$1"
  local variant="$2"
  case "${variant}" in
    direct) echo "${component}-${DEPLOY_NAMESPACE}" ;;
    flux) echo "${component}-${DEPLOY_NAMESPACE}-flux" ;;
    argo) echo "${component}-${DEPLOY_NAMESPACE}-argo" ;;
    *)
      echo "Unknown deployment variant: ${variant}" >&2
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
    --label "Stack=${STACK}" \
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
    --label "Stack=${STACK}" \
    --label "GPUUser=$(is_gpu_user "${component}")" \
    --label "Role=$(component_role "${component}")" \
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
  cub unit data --space "${space}" "${unit}" >"${output_path}"
}

get_unit_json() {
  local space="$1"
  local unit="$2"
  cub unit get --space "${space}" -o json "${unit}"
}

get_space_json() {
  local space="$1"
  cub space get -o json "${space}"
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

get_target_json() {
  local target_ref="$1"

  if [[ -z "${target_ref}" ]]; then
    return 1
  fi

  if [[ "${target_ref}" == */* ]]; then
    local target_space="${target_ref%%/*}"
    local target_slug="${target_ref##*/}"
    cub target list --space "*" -o json \
      | jq --arg space "${target_space}" --arg target "${target_slug}" '
          [ .[] | select(.Space.Slug == $space and .Target.Slug == $target) ][0]
        '
    return
  fi

  cub target list --space "*" -o json \
    | jq --arg target "${target_ref}" '
        [ .[] | select(.Target.Slug == $target) ][0]
      '
}

get_target_provider_type() {
  local target_ref="$1"
  get_target_json "${target_ref}" | jq -r '.Target.ProviderType // ""'
}

deployment_variant_for_provider_type() {
  local provider_type="$1"
  case "${provider_type}" in
    Kubernetes) echo "direct" ;;
    FluxOCI|FluxOCIWriter) echo "flux" ;;
    ArgoCDOCI) echo "argo" ;;
    *) echo "" ;;
  esac
}

supported_target_description() {
  cat <<'EOF_TARGETS'
Supported live target provider types for this example:
- Kubernetes -> direct deployment variant
- FluxOCI or FluxOCIWriter -> Flux deployment variant
- ArgoCDOCI -> Argo deployment variant
EOF_TARGETS
}

assert_supported_live_target() {
  local target_ref="${1:-}"
  local provider_type variant

  if [[ -z "${target_ref}" ]]; then
    return 0
  fi

  provider_type="$(get_target_provider_type "${target_ref}" 2>/dev/null || true)"
  variant="$(deployment_variant_for_provider_type "${provider_type}")"
  if [[ -n "${variant}" ]]; then
    return 0
  fi

  cat >&2 <<EOF_TARGET
Target ${target_ref} uses provider type ${provider_type:-<unknown>}, which this example does not support.

$(supported_target_description)
EOF_TARGET
  exit 1
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
  local target_display
  if [[ $# -eq 0 ]]; then
    target_display="<none>"
  else
    target_display="$*"
  fi
  cat <<EOF_PLAN
This is a read-only setup plan for ${EXAMPLE_NAME}.
Nothing will be created or mutated.

Inputs:
- prefix: $(state_prefix)
- deploy namespace: ${DEPLOY_NAMESPACE}
- deploy region: ${DEPLOY_REGION}
- stack: ${STACK}
- targets: ${target_display}

This example materializes one shared Enterprise RAG recipe and three deployment variants at the leaf.
The recipe layer is the app-level intent; the deployment units are the leaf deployment variants.

Spaces that ./setup.sh will create:
- $(base_space)
- $(platform_space)
- $(accelerator_space)
- $(profile_space)
- $(recipe_space)
- $(deploy_space)        # direct deployment variant
- $(flux_deploy_space)   # Flux deployment variant
- $(argo_deploy_space)   # Argo deployment variant

Components and variant chains:
- rag-server: $(base_space)/$(unit_name rag-server base) -> $(platform_space)/$(unit_name rag-server platform) -> $(accelerator_space)/$(unit_name rag-server accelerator) -> $(profile_space)/$(unit_name rag-server profile) -> $(recipe_space)/$(unit_name rag-server recipe)
  direct leaf: $(deploy_space)/$(deployment_unit_name rag-server direct)
  flux leaf: $(flux_deploy_space)/$(deployment_unit_name rag-server flux)
  argo leaf: $(argo_deploy_space)/$(deployment_unit_name rag-server argo)
- nim-llm: $(base_space)/$(unit_name nim-llm base) -> $(platform_space)/$(unit_name nim-llm platform) -> $(accelerator_space)/$(unit_name nim-llm accelerator) -> $(profile_space)/$(unit_name nim-llm profile) -> $(recipe_space)/$(unit_name nim-llm recipe)
  direct leaf: $(deploy_space)/$(deployment_unit_name nim-llm direct)
  flux leaf: $(flux_deploy_space)/$(deployment_unit_name nim-llm flux)
  argo leaf: $(argo_deploy_space)/$(deployment_unit_name nim-llm argo)
- nim-embedding: $(base_space)/$(unit_name nim-embedding base) -> $(platform_space)/$(unit_name nim-embedding platform) -> $(accelerator_space)/$(unit_name nim-embedding accelerator) -> $(profile_space)/$(unit_name nim-embedding profile) -> $(recipe_space)/$(unit_name nim-embedding recipe)
  direct leaf: $(deploy_space)/$(deployment_unit_name nim-embedding direct)
  flux leaf: $(flux_deploy_space)/$(deployment_unit_name nim-embedding flux)
  argo leaf: $(argo_deploy_space)/$(deployment_unit_name nim-embedding argo)
- vector-db: $(base_space)/$(unit_name vector-db base) -> $(platform_space)/$(unit_name vector-db platform) -> $(accelerator_space)/$(unit_name vector-db accelerator) -> $(profile_space)/$(unit_name vector-db profile) -> $(recipe_space)/$(unit_name vector-db recipe)
  direct leaf: $(deploy_space)/$(deployment_unit_name vector-db direct)
  flux leaf: $(flux_deploy_space)/$(deployment_unit_name vector-db flux)
  argo leaf: $(argo_deploy_space)/$(deployment_unit_name vector-db argo)

Layer mutations:
- platform: set kgpu storage class and platform-level config
- accelerator: set h100 GPU resource requests and node selector on GPU-using pods; label-only on rag-server and vector-db
- profile: set MODEL_NAME, MODEL_TAG, EMBED_MODEL_NAME, EMBED_DIM, MAX_BATCH_SIZE; STACK-aware image refs
- recipe: wire LLM_HOST, EMBEDDING_HOST, VECTOR_DB_HOST, RAG_TOP_K, PROMPT_TEMPLATE, GUARDRAIL_POLICY for enterprise-rag
- deployment variants: set namespace=${DEPLOY_NAMESPACE}, TENANT, REGION, CLUSTER on all leaves; STACK=ollama re-points rag-server to host Ollama

Recipe manifest:
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}
- source template: ${RECIPE_BASE_TEMPLATE}
- rendered output: ${STATE_DIR}/${RECIPE_MANIFEST_UNIT}.rendered.yaml

Core cub commands:
- cub space create
- cub unit create <unit> <file>
- cub unit create --upstream-space ... --upstream-unit ...
- cub function do set-env
- cub function do set-image-reference
- cub function do set-namespace
- cub unit set-target <kubernetes target>   # for direct variant
- cub unit set-target <fluxoci target>      # for flux variant
- cub unit set-target <argocdoci target>    # for argo variant
EOF_PLAN
}

show_setup_plan_json() {
  local target_refs_json='[]'
  if [[ $# -gt 0 ]]; then
    target_refs_json="$(printf '%s\n' "$@" | jq -R . | jq -s '.')"
  fi
  local components_json
  components_json="$(_components_json)"
  jq -n \
    --arg example "${EXAMPLE_NAME}" \
    --arg prefix "$(state_prefix)" \
    --arg namespace "${DEPLOY_NAMESPACE}" \
    --arg region "${DEPLOY_REGION}" \
    --arg stack "${STACK}" \
    --argjson targetRefs "${target_refs_json}" \
    --arg baseSpace "$(base_space)" \
    --arg platformSpace "$(platform_space)" \
    --arg acceleratorSpace "$(accelerator_space)" \
    --arg profileSpace "$(profile_space)" \
    --arg recipeSpace "$(recipe_space)" \
    --arg directDeploySpace "$(deploy_space)" \
    --arg fluxDeploySpace "$(flux_deploy_space)" \
    --arg argoDeploySpace "$(argo_deploy_space)" \
    --argjson components "${components_json}" \
    --arg manifestUnit "${RECIPE_MANIFEST_UNIT}" \
    --arg manifestTemplate "${RECIPE_BASE_TEMPLATE}" \
    --arg manifestRendered "${STATE_DIR}/${RECIPE_MANIFEST_UNIT}.rendered.yaml" \
    '{
      example: $example,
      mode: "setup-plan",
      mutates: false,
      prefix: $prefix,
      namespace: $namespace,
      region: $region,
      stack: $stack,
      targetRefs: $targetRefs,
      spaces: [
        {stage: "base", space: $baseSpace},
        {stage: "platform", space: $platformSpace},
        {stage: "accelerator", space: $acceleratorSpace},
        {stage: "profile", space: $profileSpace},
        {stage: "recipe", space: $recipeSpace},
        {stage: "deployment", variant: "direct", space: $directDeploySpace},
        {stage: "deployment", variant: "flux", space: $fluxDeploySpace},
        {stage: "deployment", variant: "argo", space: $argoDeploySpace}
      ],
      components: $components,
      recipeManifest: {
        space: $recipeSpace,
        unit: $manifestUnit,
        template: $manifestTemplate,
        renderedOutput: $manifestRendered
      },
      commands: [
        "cub space create",
        "cub unit create <unit> <file>",
        "cub unit create --upstream-space ... --upstream-unit ...",
        "cub function do set-env",
        "cub function do set-image-reference",
        "cub function do set-namespace",
        "cub unit set-target <kubernetes target>",
        "cub unit set-target <fluxoci target>",
        "cub unit set-target <argocdoci target>"
      ]
    }'
}

_components_json() {
  local out='[]'
  local component
  for component in "${COMPONENTS[@]}"; do
    out="$(jq -n \
      --argjson acc "${out}" \
      --arg component "${component}" \
      --arg role "$(component_role "${component}")" \
      --argjson gpuUser "$(is_gpu_user "${component}")" \
      --arg source "$(source_yaml_for "${component}")" \
      --arg baseSpace "$(base_space)" \
      --arg platformSpace "$(platform_space)" \
      --arg acceleratorSpace "$(accelerator_space)" \
      --arg profileSpace "$(profile_space)" \
      --arg recipeSpace "$(recipe_space)" \
      --arg directDeploySpace "$(deploy_space)" \
      --arg fluxDeploySpace "$(flux_deploy_space)" \
      --arg argoDeploySpace "$(argo_deploy_space)" \
      --arg baseUnit "$(unit_name "${component}" base)" \
      --arg platformUnit "$(unit_name "${component}" platform)" \
      --arg acceleratorUnit "$(unit_name "${component}" accelerator)" \
      --arg profileUnit "$(unit_name "${component}" profile)" \
      --arg recipeUnit "$(unit_name "${component}" recipe)" \
      --arg directUnit "$(deployment_unit_name "${component}" direct)" \
      --arg fluxUnit "$(deployment_unit_name "${component}" flux)" \
      --arg argoUnit "$(deployment_unit_name "${component}" argo)" \
      '$acc + [{
        component: $component,
        role: $role,
        gpuUser: $gpuUser,
        source: $source,
        chain: [
          {stage: "base", space: $baseSpace, unit: $baseUnit},
          {stage: "platform", space: $platformSpace, unit: $platformUnit},
          {stage: "accelerator", space: $acceleratorSpace, unit: $acceleratorUnit},
          {stage: "profile", space: $profileSpace, unit: $profileUnit},
          {stage: "recipe", space: $recipeSpace, unit: $recipeUnit}
        ],
        deploymentVariants: [
          {variant: "direct", space: $directDeploySpace, unit: $directUnit},
          {variant: "flux", space: $fluxDeploySpace, unit: $fluxUnit},
          {variant: "argo", space: $argoDeploySpace, unit: $argoUnit}
        ]
      }]')"
  done
  echo "${out}"
}

render_recipe_manifest() {
  local output_path="$1"
  local direct_target_ref="$2"
  local flux_target_ref="$3"
  local argo_target_ref="${4:-}"
  local effective_direct_target_ref="${direct_target_ref:-unset}"
  local effective_flux_target_ref="${flux_target_ref:-unset}"
  local effective_argo_target_ref="${argo_target_ref:-unset}"
  local sed_args=()
  local component stage space unit revision hash variant deploy_variant_space deploy_variant_unit

  sed_args+=(
    -e "s|confighubplaceholder-chain-name|${CHAIN_LABEL}|g"
    -e "s|confighubplaceholder-example-name|${EXAMPLE_NAME}|g"
    -e "s|confighubplaceholder-chain-prefix|$(state_prefix)|g"
    -e "s|confighubplaceholder-app-name|${APP_NAME}|g"
    -e "s|confighubplaceholder-stack|${STACK}|g"
    -e "s|confighubplaceholder-direct-target-ref|${effective_direct_target_ref}|g"
    -e "s|confighubplaceholder-direct-bundle-hint|$(bundle_hint_from_target_ref "${direct_target_ref}")|g"
    -e "s|confighubplaceholder-flux-target-ref|${effective_flux_target_ref}|g"
    -e "s|confighubplaceholder-flux-bundle-hint|$(bundle_hint_from_target_ref "${flux_target_ref}")|g"
    -e "s|confighubplaceholder-argo-target-ref|${effective_argo_target_ref}|g"
    -e "s|confighubplaceholder-argo-bundle-hint|$(bundle_hint_from_target_ref "${argo_target_ref}")|g"
  )

  for component in "${COMPONENTS[@]}"; do
    for stage in "${CHAIN_STAGES[@]}"; do
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
    for variant in "${DEPLOY_VARIANTS[@]}"; do
      deploy_variant_space="$(deploy_space_for_variant "${variant}")"
      deploy_variant_unit="$(deployment_unit_name "${component}" "${variant}")"
      revision="$(get_unit_field "${deploy_variant_space}" "${deploy_variant_unit}" HeadRevisionNum)"
      hash="$(get_unit_field "${deploy_variant_space}" "${deploy_variant_unit}" DataHash || true)"
      sed_args+=(
        -e "s|confighubplaceholder-${component}-deployment-${variant}-space|${deploy_variant_space}|g"
        -e "s|confighubplaceholder-${component}-deployment-${variant}-unit|${deploy_variant_unit}|g"
        -e "s|confighubplaceholder-${component}-deployment-${variant}-revision|${revision}|g"
        -e "s|confighubplaceholder-${component}-deployment-${variant}-hash|${hash}|g"
      )
    done
  done

  sed "${sed_args[@]}" "${RECIPE_BASE_TEMPLATE}" >"${output_path}"
}

refresh_recipe_manifest_unit() {
  local direct_target_ref="$1"
  local flux_target_ref="$2"
  local argo_target_ref="${3:-}"
  local rendered_manifest="${STATE_DIR}/${RECIPE_MANIFEST_UNIT}.rendered.yaml"
  ensure_state_dir
  render_recipe_manifest "${rendered_manifest}" "${direct_target_ref}" "${flux_target_ref}" "${argo_target_ref}"

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
  local container space unit
  container="${component}"
  space="$(platform_space)"
  unit="$(unit_name "${component}" platform)"

  case "${component}" in
    rag-server)
      cub function do set-env "${container}" "STORAGE_CLASS=gp3" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "INGRESS_CLASS=alb" --space "${space}" --unit "${unit}"
      ;;
    nim-llm|nim-embedding)
      cub function do set-env "${container}" "STORAGE_CLASS=gp3" --space "${space}" --unit "${unit}"
      ;;
    vector-db)
      cub function do set-env "${container}" "STORAGE_CLASS=gp3" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "STORAGE_SIZE=200Gi" --space "${space}" --unit "${unit}"
      ;;
  esac
}

apply_accelerator_mutations() {
  local component="$1"
  local container space unit
  container="${component}"
  space="$(accelerator_space)"
  unit="$(unit_name "${component}" accelerator)"

  cub function do set-env "${container}" "ACCELERATOR=${ACCELERATOR_VALUE}" --space "${space}" --unit "${unit}"

  case "${component}" in
    nim-llm)
      cub function do set-env "${container}" "NODE_SELECTOR=nvidia-h100" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "GPU_MEMORY=80Gi" --space "${space}" --unit "${unit}"
      ;;
    nim-embedding)
      cub function do set-env "${container}" "NODE_SELECTOR=nvidia-h100" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "GPU_MEMORY=24Gi" --space "${space}" --unit "${unit}"
      ;;
  esac
}

apply_profile_mutations() {
  local component="$1"
  local container space unit
  container="${component}"
  space="$(profile_space)"
  unit="$(unit_name "${component}" profile)"

  cub function do set-env "${container}" "STACK=${STACK}" --space "${space}" --unit "${unit}"
  cub function do set-env "${container}" "MODEL_PROFILE=${PROFILE_VALUE}" --space "${space}" --unit "${unit}"

  # Note: image refs are NOT swapped at the profile layer.
  # set-image-reference only accepts ":<tag>" or "@<digest>", not full image references.
  # For STACK=nim, manually edit profile-layer units to point at nvcr.io/nim/... images
  # before applying. For STACK=ollama, the in-cluster nim-llm and nim-embedding pods are
  # bypassed (rag-server reaches host Ollama via host.docker.internal), so the stub
  # images in the base manifests are functionally fine.

  case "${component}" in
    rag-server)
      cub function do set-env "${container}" "MODEL_NAME=${DEFAULT_LLM_MODEL_NAME}" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "EMBED_MODEL_NAME=${DEFAULT_EMBED_MODEL_NAME}" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "EMBED_DIM=${DEFAULT_EMBED_DIM}" --space "${space}" --unit "${unit}"
      ;;
    nim-llm)
      cub function do set-env "${container}" "MODEL_NAME=${DEFAULT_LLM_MODEL_NAME}" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "MODEL_TAG=${DEFAULT_LLM_MODEL_TAG}" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "MAX_BATCH_SIZE=8" --space "${space}" --unit "${unit}"
      ;;
    nim-embedding)
      cub function do set-env "${container}" "EMBED_MODEL_NAME=${DEFAULT_EMBED_MODEL_NAME}" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "EMBED_DIM=${DEFAULT_EMBED_DIM}" --space "${space}" --unit "${unit}"
      ;;
    vector-db)
      cub function do set-env "${container}" "EMBED_DIM=${DEFAULT_EMBED_DIM}" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "INDEX_TYPE=HNSW" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "METRIC=cosine" --space "${space}" --unit "${unit}"
      ;;
  esac
}

apply_recipe_mutations() {
  local component="$1"
  local container space unit
  container="${component}"
  space="$(recipe_space)"
  unit="$(unit_name "${component}" recipe)"

  case "${component}" in
    rag-server)
      cub function do set-env "${container}" "LLM_HOST=nim-llm.${DEPLOY_NAMESPACE}.svc.cluster.local:8000" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "EMBEDDING_HOST=nim-embedding.${DEPLOY_NAMESPACE}.svc.cluster.local:8001" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "VECTOR_DB_HOST=vector-db.${DEPLOY_NAMESPACE}.svc.cluster.local:6333" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "RAG_TOP_K=5" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "PROMPT_TEMPLATE=enterprise-default" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "GUARDRAIL_POLICY=enterprise-default" --space "${space}" --unit "${unit}"
      cub function do set-env "${container}" "RAG_USE_CASE=${RECIPE_VALUE}" --space "${space}" --unit "${unit}"
      ;;
    nim-llm|nim-embedding)
      cub function do set-env "${container}" "RAG_USE_CASE=${RECIPE_VALUE}" --space "${space}" --unit "${unit}"
      ;;
    vector-db)
      cub function do set-env "${container}" "COLLECTION_PREFIX=${RECIPE_VALUE}" --space "${space}" --unit "${unit}"
      ;;
  esac
}

apply_deploy_mutations() {
  local component="$1"
  local variant="${2:-direct}"
  local deploy_variant_space deploy_variant_unit container

  deploy_variant_space="$(deploy_space_for_variant "${variant}")"
  deploy_variant_unit="$(deployment_unit_name "${component}" "${variant}")"
  container="${component}"

  cub function do set-namespace "${DEPLOY_NAMESPACE}" --space "${deploy_variant_space}" --unit "${deploy_variant_unit}"
  cub function do set-env "${container}" "TENANT=${DEPLOY_NAMESPACE}" --space "${deploy_variant_space}" --unit "${deploy_variant_unit}"
  cub function do set-env "${container}" "CLUSTER=${DEPLOY_NAMESPACE}" --space "${deploy_variant_space}" --unit "${deploy_variant_unit}"

  case "${component}" in
    rag-server)
      cub function do set-env "${container}" "REGION=${DEPLOY_REGION}" --space "${deploy_variant_space}" --unit "${deploy_variant_unit}"
      # STACK=ollama: redirect rag-server's LLM and embedding hosts to host Ollama (Metal-accelerated).
      # This is the deployment-layer override that proves Story 3 (per-tenant endpoint).
      if [[ "${STACK}" == "ollama" ]]; then
        cub function do set-env "${container}" "LLM_HOST=$(llm_host_for ollama)" --space "${deploy_variant_space}" --unit "${deploy_variant_unit}"
        cub function do set-env "${container}" "EMBEDDING_HOST=$(embedding_host_for ollama)" --space "${deploy_variant_space}" --unit "${deploy_variant_unit}"
      fi
      ;;
  esac
}

set_target_for_deploy_variant() {
  local variant="$1"
  local target_ref="$2"
  local component
  for component in "${COMPONENTS[@]}"; do
    cub unit set-target "${target_ref}" --space "$(deploy_space_for_variant "${variant}")" --unit "$(deployment_unit_name "${component}" "${variant}")"
  done
}

remember_target_ref_for_variant() {
  local target_ref="$1"
  local provider_type variant

  provider_type="$(get_target_provider_type "${target_ref}")"
  variant="$(deployment_variant_for_provider_type "${provider_type}")"
  case "${variant}" in
    direct) DIRECT_TARGET_REF="${target_ref}" ;;
    flux) FLUX_TARGET_REF="${target_ref}" ;;
    argo) ARGO_TARGET_REF="${target_ref}" ;;
    *)
      echo "Unsupported target provider type: ${provider_type}" >&2
      exit 1
      ;;
  esac
}

set_target_for_compatible_units() {
  local target_ref="$1"
  local provider_type variant

  provider_type="$(get_target_provider_type "${target_ref}")"
  variant="$(deployment_variant_for_provider_type "${provider_type}")"
  case "${variant}" in
    direct|flux|argo)
      set_target_for_deploy_variant "${variant}" "${target_ref}"
      remember_target_ref_for_variant "${target_ref}"
      ;;
    *)
      echo "Unsupported target provider type: ${provider_type}" >&2
      exit 1
      ;;
  esac
}

show_summary() {
  local recipe_space_url deploy_space_url flux_deploy_space_url argo_deploy_space_url
  local recipe_unit_url rag_unit_url
  recipe_space_url="$(gui_space_url "$(recipe_space)")"
  deploy_space_url="$(gui_space_url "$(deploy_space)")"
  flux_deploy_space_url="$(gui_space_url "$(flux_deploy_space)")"
  argo_deploy_space_url="$(gui_space_url "$(argo_deploy_space)")"
  recipe_unit_url="$(gui_unit_url "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}")"
  rag_unit_url="$(gui_unit_url "$(deploy_space)" "$(deployment_unit_name rag-server direct)")"
  cat <<EOF_SUMMARY
Created Enterprise RAG Blueprint chain with prefix: $(state_prefix)
Stack: ${STACK}

Spaces:
- $(base_space)
- $(platform_space)
- $(accelerator_space)
- $(profile_space)
- $(recipe_space)
- $(deploy_space)
- $(flux_deploy_space)
- $(argo_deploy_space)

Components: rag-server, nim-llm, nim-embedding, vector-db
Each component has 5 chain units (base, platform, accelerator, profile, recipe) plus 3 deployment variants (direct, flux, argo).

Recipe manifest unit: $(recipe_space)/${RECIPE_MANIFEST_UNIT}

Model:
- shared recipe/app: ${CHAIN_LABEL}
- deployment namespace: ${DEPLOY_NAMESPACE}
- region: ${DEPLOY_REGION}
- stack: ${STACK}
- supported live targets:
  - Kubernetes -> direct variant
  - FluxOCI / FluxOCIWriter -> flux variant
  - ArgoCDOCI -> argo variant
- direct target: ${DIRECT_TARGET_REF:-<unset>}
- flux target: ${FLUX_TARGET_REF:-<unset>}
- argo target: ${ARGO_TARGET_REF:-<unset>}

GUI:
- Recipe space: ${recipe_space_url}
- Direct deploy space: ${deploy_space_url}
- Flux deploy space: ${flux_deploy_space_url}
- Argo deploy space: ${argo_deploy_space_url}
- Recipe manifest: ${recipe_unit_url}
- Direct rag-server deployment unit: ${rag_unit_url}

Logs:
- Setup log: $(current_log_path setup)
- Set-target log: $(current_log_path set-target)
- Verify log: $(current_log_path verify)
- Cleanup log: $(current_log_path cleanup)

Next steps:
1. ./verify.sh
2. ./seed-initiatives.sh
3. ./set-target.sh <kubernetes-target>        # binds the direct deployment variant
4. cub unit approve --space $(deploy_space) $(deployment_unit_name rag-server direct)
   cub unit apply   --space $(deploy_space) $(deployment_unit_name rag-server direct)
   (repeat for nim-llm, nim-embedding, vector-db)
5. ./query.sh "What is the capital of France?"   # STACK=ollama only
EOF_SUMMARY
}

all_spaces() {
  printf '%s\n' \
    "$(base_space)" \
    "$(platform_space)" \
    "$(accelerator_space)" \
    "$(profile_space)" \
    "$(recipe_space)" \
    "$(deploy_space)" \
    "$(flux_deploy_space)" \
    "$(argo_deploy_space)"
}

all_unit_refs() {
  local component stage variant
  for component in "${COMPONENTS[@]}"; do
    for stage in "${CHAIN_STAGES[@]}"; do
      printf '%s/%s\n' "$(space_for_stage "${stage}")" "$(unit_name "${component}" "${stage}")"
    done
    for variant in "${DEPLOY_VARIANTS[@]}"; do
      printf '%s/%s\n' "$(deploy_space_for_variant "${variant}")" "$(deployment_unit_name "${component}" "${variant}")"
    done
  done
  printf '%s/%s\n' "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}"
}
