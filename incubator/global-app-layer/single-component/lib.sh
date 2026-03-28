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
DEPLOY_FLUX_SPACE_SUFFIX="deploy-cluster-a-flux"

BASE_UNIT="backend-base"
REGION_UNIT="backend-us"
ROLE_UNIT="backend-us-staging"
RECIPE_UNIT="backend-recipe-us-staging"
DEPLOY_UNIT="backend-cluster-a"
DEPLOY_FLUX_UNIT="backend-cluster-a-flux"
RECIPE_MANIFEST_UNIT="recipe-us-staging"

DEPLOY_VARIANTS=(direct flux)

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
  : "${DIRECT_TARGET_REF:=${TARGET_REF:-}}"
  : "${FLUX_TARGET_REF:=}"
}

save_state() {
  local prefix="$1"
  local direct_target_ref="${2:-}"
  local flux_target_ref="${3:-}"

  ensure_state_dir
  printf 'PREFIX=%q\nTARGET_REF=%q\nDIRECT_TARGET_REF=%q\nFLUX_TARGET_REF=%q\nDEPLOY_NAMESPACE=%q\n' \
    "${prefix}" "${direct_target_ref}" "${direct_target_ref}" "${flux_target_ref}" "${DEPLOY_NAMESPACE}" >"${STATE_FILE}"
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

flux_deploy_space() {
  space_name "${DEPLOY_FLUX_SPACE_SUFFIX}"
}

deploy_space_for_variant() {
  local variant="$1"
  case "${variant}" in
    direct) deploy_space ;;
    flux) flux_deploy_space ;;
    *)
      echo "Unknown deployment variant: ${variant}" >&2
      exit 1
      ;;
  esac
}

deployment_unit_name() {
  local variant="$1"
  case "${variant}" in
    direct) echo "${DEPLOY_UNIT}" ;;
    flux) echo "${DEPLOY_FLUX_UNIT}" ;;
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

get_target_json() {
  local target_ref="$1"

  if [[ -z "${target_ref}" ]]; then
    return 1
  fi

  if [[ "${target_ref}" == */* ]]; then
    local target_space="${target_ref%%/*}"
    local target_slug="${target_ref##*/}"
    cub target list --space "*" --json \
      | jq --arg space "${target_space}" --arg target "${target_slug}" '
          [ .[] | select(.Space.Slug == $space and .Target.Slug == $target) ][0]
        '
    return
  fi

  cub target list --space "*" --json \
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
    *) echo "" ;;
  esac
}

supported_target_description() {
  cat <<'EOF_TARGETS'
Supported live target provider types for this example:
- Kubernetes -> direct deployment variant
- FluxOCI or FluxOCIWriter -> Flux deployment variant
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

set_target_for_compatible_units() {
  local target_ref="$1"
  local provider_type variant

  provider_type="$(get_target_provider_type "${target_ref}")"
  variant="$(deployment_variant_for_provider_type "${provider_type}")"
  case "${variant}" in
    direct)
      cub unit set-target "${target_ref}" --space "$(deploy_space)" --unit "${DEPLOY_UNIT}"
      cub unit set-target "${target_ref}" --space "$(deploy_space)" --unit "${DEPLOY_STUB_UNIT}"
      DIRECT_TARGET_REF="${target_ref}"
      ;;
    flux)
      cub unit set-target "${target_ref}" --space "$(flux_deploy_space)" --unit "${DEPLOY_FLUX_UNIT}"
      cub unit set-target "${target_ref}" --space "$(flux_deploy_space)" --unit "postgres-stub-flux"
      FLUX_TARGET_REF="${target_ref}"
      ;;
    *)
      echo "Unsupported target provider type: ${provider_type}" >&2
      exit 1
      ;;
  esac
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
- targets: ${target_display}

This example materializes one recipe chain and two deployment variants at the leaf.
The shared recipe is the app-level intent; the deployment units are the leaf deployment variants.

Spaces that ./setup.sh will create:
- $(base_space)
- $(region_space)
- $(role_space)
- $(recipe_space)
- $(deploy_space)        # direct deployment variant
- $(flux_deploy_space)   # Flux deployment variant

Variant chain:
- $(base_space)/${BASE_UNIT}
- $(region_space)/${REGION_UNIT}
- $(role_space)/${ROLE_UNIT}
- $(recipe_space)/${RECIPE_UNIT}
- $(deploy_space)/${DEPLOY_UNIT} (direct)
- $(flux_deploy_space)/${DEPLOY_FLUX_UNIT} (flux)

Dependency stub:
- $(deploy_space)/${DEPLOY_STUB_UNIT} (direct)
- $(flux_deploy_space)/postgres-stub-flux (flux)

Layer mutations:
- region: set backend REGION=${REGION_VALUE} and regional ingress host
- role: set ROLE=${ROLE_VALUE}, replicas=2, and LOG_LEVEL=info
- recipe: set CHAT_TITLE for the resolved recipe
- deployment (both variants): set namespace=${DEPLOY_NAMESPACE}, CLUSTER=${DEPLOY_NAMESPACE}, and deployment ingress host
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
- cub unit set-target <kubernetes-target>   # for direct variant
- cub unit set-target <fluxoci-target>      # for flux variant
EOF_PLAN
}

show_setup_plan_json() {
  local target_refs_json='[]'
  if [[ $# -gt 0 ]]; then
    target_refs_json="$(printf '%s\n' "$@" | jq -R . | jq -s '.')"
  fi
  jq -n \
    --arg example "${EXAMPLE_NAME}" \
    --arg prefix "$(state_prefix)" \
    --arg namespace "${DEPLOY_NAMESPACE}" \
    --argjson targetRefs "${target_refs_json}" \
    --arg source "${SOURCE_BACKEND_YAML}" \
    --arg stubSource "${POSTGRES_STUB_YAML}" \
    --arg baseSpace "$(base_space)" \
    --arg regionSpace "$(region_space)" \
    --arg roleSpace "$(role_space)" \
    --arg recipeSpace "$(recipe_space)" \
    --arg directDeploySpace "$(deploy_space)" \
    --arg fluxDeploySpace "$(flux_deploy_space)" \
    --arg baseUnit "${BASE_UNIT}" \
    --arg regionUnit "${REGION_UNIT}" \
    --arg roleUnit "${ROLE_UNIT}" \
    --arg recipeUnit "${RECIPE_UNIT}" \
    --arg directDeployUnit "${DEPLOY_UNIT}" \
    --arg fluxDeployUnit "${DEPLOY_FLUX_UNIT}" \
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
      targetRefs: $targetRefs,
      spaces: [
        {stage: "base", space: $baseSpace},
        {stage: "region", space: $regionSpace},
        {stage: "role", space: $roleSpace},
        {stage: "recipe", space: $recipeSpace},
        {stage: "deployment", variant: "direct", space: $directDeploySpace},
        {stage: "deployment", variant: "flux", space: $fluxDeploySpace}
      ],
      components: [
        {
          component: "backend",
          source: $source,
          chain: [
            {stage: "base", space: $baseSpace, unit: $baseUnit},
            {stage: "region", space: $regionSpace, unit: $regionUnit},
            {stage: "role", space: $roleSpace, unit: $roleUnit},
            {stage: "recipe", space: $recipeSpace, unit: $recipeUnit}
          ],
          deploymentVariants: [
            {variant: "direct", space: $directDeploySpace, unit: $directDeployUnit},
            {variant: "flux", space: $fluxDeploySpace, unit: $fluxDeployUnit}
          ],
          mutations: [
            {stage: "region", summary: "set backend REGION and regional ingress host"},
            {stage: "role", summary: "set ROLE, replicas, and LOG_LEVEL"},
            {stage: "recipe", summary: "set CHAT_TITLE"},
            {stage: "deploymentVariants", summary: "set namespace, CLUSTER, and deployment ingress host on both variants"}
          ]
        },
        {
          component: "postgres-stub",
          source: $stubSource,
          stub: true,
          deploymentVariants: [
            {variant: "direct", space: $directDeploySpace, unit: $stubUnit},
            {variant: "flux", space: $fluxDeploySpace, unit: "postgres-stub-flux"}
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
      commands: [
        "cub space create",
        "cub unit create <unit> <file>",
        "cub unit create --upstream-space ... --upstream-unit ...",
        "cub function do set-env",
        "cub function do set-replicas",
        "cub function do set-string-path",
        "cub function do set-namespace",
        "cub unit set-target <kubernetes target>",
        "cub unit set-target <fluxoci target>"
      ]
    }'
}

deploy_backend_hostname() {
  echo "backend.${DEPLOY_NAMESPACE}.demo.confighub.local"
}

render_recipe_manifest() {
  local output_path="$1"
  local direct_target_ref="$2"
  local flux_target_ref="$3"
  local effective_direct_target_ref="${direct_target_ref:-unset}"
  local effective_flux_target_ref="${flux_target_ref:-unset}"

  local base_rev region_rev role_rev recipe_rev deploy_rev flux_deploy_rev
  local base_hash region_hash role_hash recipe_hash deploy_hash flux_deploy_hash
  base_rev="$(get_unit_field "$(base_space)" "${BASE_UNIT}" HeadRevisionNum)"
  region_rev="$(get_unit_field "$(region_space)" "${REGION_UNIT}" HeadRevisionNum)"
  role_rev="$(get_unit_field "$(role_space)" "${ROLE_UNIT}" HeadRevisionNum)"
  recipe_rev="$(get_unit_field "$(recipe_space)" "${RECIPE_UNIT}" HeadRevisionNum)"
  deploy_rev="$(get_unit_field "$(deploy_space)" "${DEPLOY_UNIT}" HeadRevisionNum)"
  flux_deploy_rev="$(get_unit_field "$(flux_deploy_space)" "${DEPLOY_FLUX_UNIT}" HeadRevisionNum)"

  base_hash="$(get_unit_field "$(base_space)" "${BASE_UNIT}" DataHash || true)"
  region_hash="$(get_unit_field "$(region_space)" "${REGION_UNIT}" DataHash || true)"
  role_hash="$(get_unit_field "$(role_space)" "${ROLE_UNIT}" DataHash || true)"
  recipe_hash="$(get_unit_field "$(recipe_space)" "${RECIPE_UNIT}" DataHash || true)"
  deploy_hash="$(get_unit_field "$(deploy_space)" "${DEPLOY_UNIT}" DataHash || true)"
  flux_deploy_hash="$(get_unit_field "$(flux_deploy_space)" "${DEPLOY_FLUX_UNIT}" DataHash || true)"

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
    -e "s|confighubplaceholder-flux-deploy-space|$(flux_deploy_space)|g" \
    -e "s|confighubplaceholder-flux-deploy-unit|${DEPLOY_FLUX_UNIT}|g" \
    -e "s|confighubplaceholder-flux-deploy-revision|${flux_deploy_rev}|g" \
    -e "s|confighubplaceholder-flux-deploy-hash|${flux_deploy_hash}|g" \
    -e "s|confighubplaceholder-direct-target-ref|${effective_direct_target_ref}|g" \
    -e "s|confighubplaceholder-direct-bundle-hint|$(bundle_hint_from_target_ref "${direct_target_ref}")|g" \
    -e "s|confighubplaceholder-flux-target-ref|${effective_flux_target_ref}|g" \
    -e "s|confighubplaceholder-flux-bundle-hint|$(bundle_hint_from_target_ref "${flux_target_ref}")|g" \
    -e "s|confighubplaceholder-target-ref|${effective_direct_target_ref}|g" \
    -e "s|confighubplaceholder-bundle-hint|$(bundle_hint_from_target_ref "${direct_target_ref}")|g" \
    "${RECIPE_BASE_TEMPLATE}" >"${output_path}"
}

refresh_recipe_manifest_unit() {
  local direct_target_ref="$1"
  local flux_target_ref="$2"
  local rendered_manifest="${STATE_DIR}/recipe-us-staging.rendered.yaml"
  ensure_state_dir
  render_recipe_manifest "${rendered_manifest}" "${direct_target_ref}" "${flux_target_ref}"

  if unit_exists "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}"; then
    cub unit update --space "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}" "${rendered_manifest}"
  else
    _mapfile manifest_labels < <(label_args recipe-manifest)
    cub unit create --space "$(recipe_space)" -t AppConfig/YAML \
      "${RECIPE_MANIFEST_UNIT}" "${rendered_manifest}" "${manifest_labels[@]}"
  fi
}

show_summary() {
  local recipe_space_url deploy_space_url flux_deploy_space_url recipe_unit_url deploy_unit_url flux_deploy_unit_url
  recipe_space_url="$(gui_space_url "$(recipe_space)")"
  deploy_space_url="$(gui_space_url "$(deploy_space)")"
  flux_deploy_space_url="$(gui_space_url "$(flux_deploy_space)")"
  recipe_unit_url="$(gui_unit_url "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}")"
  deploy_unit_url="$(gui_unit_url "$(deploy_space)" "${DEPLOY_UNIT}")"
  flux_deploy_unit_url="$(gui_unit_url "$(flux_deploy_space)" "${DEPLOY_FLUX_UNIT}")"
  cat <<EOF_SUMMARY
Created global-app layered chain with prefix: $(state_prefix)

Spaces:
- $(base_space)
- $(region_space)
- $(role_space)
- $(recipe_space)
- $(deploy_space)
- $(flux_deploy_space)

Units:
- $(base_space)/${BASE_UNIT}
- $(region_space)/${REGION_UNIT}
- $(role_space)/${ROLE_UNIT}
- $(recipe_space)/${RECIPE_UNIT}
- $(recipe_space)/${RECIPE_MANIFEST_UNIT}
- $(deploy_space)/${DEPLOY_UNIT}
- $(deploy_space)/${DEPLOY_STUB_UNIT}
- $(flux_deploy_space)/${DEPLOY_FLUX_UNIT}
- $(flux_deploy_space)/postgres-stub-flux

Model:
- shared recipe: ${CHAIN_LABEL}
- deployment variants:
  - direct: ${DEPLOY_UNIT}
  - flux: ${DEPLOY_FLUX_UNIT}
- supported live targets:
  - Kubernetes -> direct variant
  - FluxOCI / FluxOCIWriter -> flux variant
- direct target: ${DIRECT_TARGET_REF:-<unset>}
- flux target: ${FLUX_TARGET_REF:-<unset>}

GUI:
- Recipe space: ${recipe_space_url}
- Direct deploy space: ${deploy_space_url}
- Flux deploy space: ${flux_deploy_space_url}
- Recipe manifest: ${recipe_unit_url}
- Direct deployment unit: ${deploy_unit_url}
- Flux deployment unit: ${flux_deploy_unit_url}

Logs:
- Setup log: $(current_log_path setup)
- Set-target log: $(current_log_path set-target)
- Verify log: $(current_log_path verify)
- Cleanup log: $(current_log_path cleanup)

Next steps:
1. ./verify.sh
2. ./upgrade-chain.sh ${DEFAULT_IMAGE_TAG}
3. ./set-target.sh <kubernetes-target>        # binds the direct deployment variant
4. ./set-target.sh <fluxoci-target>           # binds the Flux deployment variant
5. cub unit approve --space $(deploy_space) ${DEPLOY_UNIT} && cub unit apply --space $(deploy_space) ${DEPLOY_UNIT}
6. cub unit approve --space $(flux_deploy_space) ${DEPLOY_FLUX_UNIT} && cub unit apply --space $(flux_deploy_space) ${DEPLOY_FLUX_UNIT}
7. Review recipe manifest: cub unit get --space $(recipe_space) --data-only ${RECIPE_MANIFEST_UNIT}
EOF_SUMMARY
}
