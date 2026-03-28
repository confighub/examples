#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
begin_log_capture verify
load_state
ensure_state_dir

for space in "$(base_space)" "$(region_space)" "$(role_space)" "$(recipe_space)" "$(deploy_space)" "$(flux_deploy_space)" "$(argo_deploy_space)"; do
  echo "==> Checking space exists: ${space}"
  cub space get "${space}" >/dev/null
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

base_file="${tmp_dir}/base.yaml"
region_file="${tmp_dir}/region.yaml"
role_file="${tmp_dir}/role.yaml"
recipe_file="${tmp_dir}/recipe.yaml"
deploy_file="${tmp_dir}/deploy.yaml"
flux_deploy_file="${tmp_dir}/flux-deploy.yaml"
argo_deploy_file="${tmp_dir}/argo-deploy.yaml"
manifest_file="${tmp_dir}/recipe-manifest.yaml"

unit_data_to_file "$(base_space)" "${BASE_UNIT}" "${base_file}"
unit_data_to_file "$(region_space)" "${REGION_UNIT}" "${region_file}"
unit_data_to_file "$(role_space)" "${ROLE_UNIT}" "${role_file}"
unit_data_to_file "$(recipe_space)" "${RECIPE_UNIT}" "${recipe_file}"
unit_data_to_file "$(deploy_space)" "${DEPLOY_UNIT}" "${deploy_file}"
unit_data_to_file "$(flux_deploy_space)" "${DEPLOY_FLUX_UNIT}" "${flux_deploy_file}"
unit_data_to_file "$(argo_deploy_space)" "${DEPLOY_ARGO_UNIT}" "${argo_deploy_file}"
unit_data_to_file "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}" "${manifest_file}"

echo "==> Verifying clone chain"
base_id="$(get_unit_field "$(base_space)" "${BASE_UNIT}" UnitID)"
region_id="$(get_unit_field "$(region_space)" "${REGION_UNIT}" UnitID)"
role_id="$(get_unit_field "$(role_space)" "${ROLE_UNIT}" UnitID)"
recipe_id="$(get_unit_field "$(recipe_space)" "${RECIPE_UNIT}" UnitID)"

actual="$(get_unit_field "$(region_space)" "${REGION_UNIT}" UpstreamUnitID)"
[[ "${actual}" == "${base_id}" ]] || { echo "Clone chain broken: region upstream ${actual} != base ${base_id}" >&2; exit 1; }

actual="$(get_unit_field "$(role_space)" "${ROLE_UNIT}" UpstreamUnitID)"
[[ "${actual}" == "${region_id}" ]] || { echo "Clone chain broken: role upstream ${actual} != region ${region_id}" >&2; exit 1; }

actual="$(get_unit_field "$(recipe_space)" "${RECIPE_UNIT}" UpstreamUnitID)"
[[ "${actual}" == "${role_id}" ]] || { echo "Clone chain broken: recipe upstream ${actual} != role ${role_id}" >&2; exit 1; }

actual="$(get_unit_field "$(deploy_space)" "${DEPLOY_UNIT}" UpstreamUnitID)"
[[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: deploy upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

actual="$(get_unit_field "$(flux_deploy_space)" "${DEPLOY_FLUX_UNIT}" UpstreamUnitID)"
[[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: flux deploy upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

actual="$(get_unit_field "$(argo_deploy_space)" "${DEPLOY_ARGO_UNIT}" UpstreamUnitID)"
[[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: argo deploy upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

echo "==> Verifying layer-specific mutations"
assert_contains "${base_file}" 'OLLAMA_ENABLED'
assert_contains "${region_file}" 'name: REGION'
assert_contains "${region_file}" 'backend.us.demo.confighub.local'
assert_contains "${role_file}" 'replicas: 2'
assert_contains "${role_file}" 'name: LOG_LEVEL'
assert_contains "${role_file}" 'name: ROLE'
assert_contains "${recipe_file}" 'Cubby Chat (US Staging Recipe)'
assert_contains "${deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
assert_contains "${deploy_file}" 'name: CLUSTER'
assert_contains "${deploy_file}" "$(deploy_backend_hostname)"

echo "==> Verifying Flux deployment variant mutations"
assert_contains "${flux_deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
assert_contains "${flux_deploy_file}" 'name: CLUSTER'
assert_contains "${flux_deploy_file}" "$(deploy_backend_hostname)"

echo "==> Verifying Argo deployment variant mutations"
assert_contains "${argo_deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
assert_contains "${argo_deploy_file}" 'name: CLUSTER'
assert_contains "${argo_deploy_file}" "$(deploy_backend_hostname)"

echo "==> Verifying explicit recipe manifest"
assert_contains "${manifest_file}" 'kind: Recipe'
assert_contains "${manifest_file}" 'name: global-app-us-staging'
assert_contains "${manifest_file}" "space: $(base_space)"
assert_contains "${manifest_file}" "space: $(deploy_space)"
assert_contains "${manifest_file}" 'bundleHint:'

if [[ -n "${DIRECT_TARGET_REF:-}" ]]; then
  echo "==> Verifying direct deployment has a target"
  get_unit_field "$(deploy_space)" "${DEPLOY_UNIT}" TargetID >/dev/null
fi

if [[ -n "${FLUX_TARGET_REF:-}" ]]; then
  echo "==> Verifying Flux deployment has a target"
  get_unit_field "$(flux_deploy_space)" "${DEPLOY_FLUX_UNIT}" TargetID >/dev/null
fi

if [[ -n "${ARGO_TARGET_REF:-}" ]]; then
  echo "==> Verifying Argo deployment has a target"
  get_unit_field "$(argo_deploy_space)" "${DEPLOY_ARGO_UNIT}" TargetID >/dev/null
fi

echo "All global-app-layer single-component checks passed."
