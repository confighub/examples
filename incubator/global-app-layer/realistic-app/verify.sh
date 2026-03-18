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

for space in "$(base_space)" "$(region_space)" "$(role_space)" "$(recipe_space)" "$(deploy_space)"; do
  echo "==> Checking space exists: ${space}"
  cub space get "${space}" >/dev/null
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

recipe_manifest_file="${tmp_dir}/recipe-manifest.yaml"
unit_data_to_file "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}" "${recipe_manifest_file}"

echo "==> Verifying clone chains"
for component in "${COMPONENTS[@]}"; do
  base_file="${tmp_dir}/${component}-base.yaml"
  region_file="${tmp_dir}/${component}-region.yaml"
  role_file="${tmp_dir}/${component}-role.yaml"
  recipe_file="${tmp_dir}/${component}-recipe.yaml"
  deploy_file="${tmp_dir}/${component}-deploy.yaml"

  unit_data_to_file "$(base_space)" "$(unit_name "${component}" base)" "${base_file}"
  unit_data_to_file "$(region_space)" "$(unit_name "${component}" region)" "${region_file}"
  unit_data_to_file "$(role_space)" "$(unit_name "${component}" role)" "${role_file}"
  unit_data_to_file "$(recipe_space)" "$(unit_name "${component}" recipe)" "${recipe_file}"
  unit_data_to_file "$(deploy_space)" "$(unit_name "${component}" deployment)" "${deploy_file}"

  base_id="$(get_unit_field "$(base_space)" "$(unit_name "${component}" base)" UnitID)"
  region_id="$(get_unit_field "$(region_space)" "$(unit_name "${component}" region)" UnitID)"
  role_id="$(get_unit_field "$(role_space)" "$(unit_name "${component}" role)" UnitID)"
  recipe_id="$(get_unit_field "$(recipe_space)" "$(unit_name "${component}" recipe)" UnitID)"

  actual="$(get_unit_field "$(region_space)" "$(unit_name "${component}" region)" UpstreamUnitID)"
  [[ "${actual}" == "${base_id}" ]] || { echo "Clone chain broken: ${component} region upstream ${actual} != base ${base_id}" >&2; exit 1; }

  actual="$(get_unit_field "$(role_space)" "$(unit_name "${component}" role)" UpstreamUnitID)"
  [[ "${actual}" == "${region_id}" ]] || { echo "Clone chain broken: ${component} role upstream ${actual} != region ${region_id}" >&2; exit 1; }

  actual="$(get_unit_field "$(recipe_space)" "$(unit_name "${component}" recipe)" UpstreamUnitID)"
  [[ "${actual}" == "${role_id}" ]] || { echo "Clone chain broken: ${component} recipe upstream ${actual} != role ${role_id}" >&2; exit 1; }

  actual="$(get_unit_field "$(deploy_space)" "$(unit_name "${component}" deployment)" UpstreamUnitID)"
  [[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: ${component} deploy upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

  case "${component}" in
    backend)
      assert_contains "${base_file}" 'image: ghcr.io/confighub/cubbychat/backend:'
      assert_contains "${base_file}" 'value: "dev"'
      assert_contains "${region_file}" 'backend.us.demo.confighub.local'
      assert_contains "${region_file}" 'value: "us"'
      assert_contains "${role_file}" 'replicas: 2'
      assert_contains "${role_file}" 'value: "staging"'
      assert_contains "${recipe_file}" 'chatdb_us_staging'
      assert_contains "${recipe_file}" 'Cubby Chat US Staging'
      assert_contains "${deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
      assert_contains "${deploy_file}" "$(deploy_hostname backend)"
      assert_contains "${deploy_file}" 'name: CLUSTER'
      assert_contains "${deploy_file}" "value: ${DEPLOY_NAMESPACE}"
      ;;
    frontend)
      assert_contains "${base_file}" 'image: ghcr.io/confighub/cubbychat/frontend:'
      assert_contains "${region_file}" 'frontend.us.demo.confighub.local'
      assert_contains "${role_file}" 'replicas: 2'
      assert_contains "${role_file}" 'name: PUBLIC_ENV'
      assert_contains "${role_file}" 'value: "staging"'
      assert_contains "${recipe_file}" 'name: RELEASE_CHANNEL'
      assert_contains "${recipe_file}" 'value: us-staging-recipe'
      assert_contains "${deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
      assert_contains "${deploy_file}" "$(deploy_hostname frontend)"
      assert_contains "${deploy_file}" 'name: CLUSTER'
      assert_contains "${deploy_file}" "value: ${DEPLOY_NAMESPACE}"
      ;;
    postgres)
      assert_contains "${base_file}" 'storage: 5Gi'
      assert_contains "${base_file}" 'image: postgres:'
      assert_contains "${region_file}" 'name: REGION'
      assert_contains "${region_file}" 'value: US'
      assert_contains "${role_file}" 'storage: 10Gi'
      assert_contains "${role_file}" 'name: ROLE'
      assert_contains "${role_file}" 'value: staging'
      assert_contains "${recipe_file}" 'chatdb_us_staging'
      assert_contains "${deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
      assert_contains "${deploy_file}" 'name: CLUSTER'
      assert_contains "${deploy_file}" "value: ${DEPLOY_NAMESPACE}"
      ;;
  esac
done

echo "==> Verifying explicit app-level recipe manifest"
assert_contains "${recipe_manifest_file}" 'kind: Recipe'
assert_contains "${recipe_manifest_file}" 'name: global-app-us-staging-realistic'
assert_contains "${recipe_manifest_file}" 'app: global-app'
assert_contains "${recipe_manifest_file}" 'name: backend'
assert_contains "${recipe_manifest_file}" 'name: frontend'
assert_contains "${recipe_manifest_file}" 'name: postgres'
assert_contains "${recipe_manifest_file}" "space: $(base_space)"
assert_contains "${recipe_manifest_file}" "space: $(deploy_space)"
assert_contains "${recipe_manifest_file}" 'bundleHint:'

if [[ -n "${TARGET_REF:-}" ]]; then
  echo "==> Verifying deployment units have a target"
  get_unit_field "$(deploy_space)" "$(unit_name backend deployment)" TargetID >/dev/null
  get_unit_field "$(deploy_space)" "$(unit_name frontend deployment)" TargetID >/dev/null
  get_unit_field "$(deploy_space)" "$(unit_name postgres deployment)" TargetID >/dev/null
fi

echo "All global-app-layer realistic-app checks passed."
