#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_python
load_state
ensure_state_dir

for space in "$(base_space)" "$(region_space)" "$(role_space)" "$(recipe_space)" "$(deploy_space)"; do
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
manifest_file="${tmp_dir}/recipe-manifest.yaml"

unit_data_to_file "$(base_space)" "${BASE_UNIT}" "${base_file}"
unit_data_to_file "$(region_space)" "${REGION_UNIT}" "${region_file}"
unit_data_to_file "$(role_space)" "${ROLE_UNIT}" "${role_file}"
unit_data_to_file "$(recipe_space)" "${RECIPE_UNIT}" "${recipe_file}"
unit_data_to_file "$(deploy_space)" "${DEPLOY_UNIT}" "${deploy_file}"
unit_data_to_file "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}" "${manifest_file}"

echo "==> Verifying clone chain"
base_id="$(get_unit_field "$(base_space)" "${BASE_UNIT}" UnitID)"
region_id="$(get_unit_field "$(region_space)" "${REGION_UNIT}" UnitID)"
role_id="$(get_unit_field "$(role_space)" "${ROLE_UNIT}" UnitID)"
recipe_id="$(get_unit_field "$(recipe_space)" "${RECIPE_UNIT}" UnitID)"

[[ "$(get_unit_field "$(region_space)" "${REGION_UNIT}" UpstreamUnitID)" == "${base_id}" ]]
[[ "$(get_unit_field "$(role_space)" "${ROLE_UNIT}" UpstreamUnitID)" == "${region_id}" ]]
[[ "$(get_unit_field "$(recipe_space)" "${RECIPE_UNIT}" UpstreamUnitID)" == "${role_id}" ]]
[[ "$(get_unit_field "$(deploy_space)" "${DEPLOY_UNIT}" UpstreamUnitID)" == "${recipe_id}" ]]

echo "==> Verifying layer-specific mutations"
assert_contains "${base_file}" 'value: "dev"'
assert_contains "${region_file}" 'value: "US"'
assert_contains "${region_file}" 'backend.us.demo.confighub.local'
assert_contains "${role_file}" 'replicas: 2'
assert_contains "${role_file}" 'name: LOG_LEVEL'
assert_contains "${role_file}" 'value: info'
assert_contains "${role_file}" 'value: "staging"'
assert_contains "${recipe_file}" 'Cubby Chat (US Staging Recipe)'
assert_contains "${deploy_file}" 'namespace: cluster-a'
assert_contains "${deploy_file}" 'name: CLUSTER'
assert_contains "${deploy_file}" 'backend.cluster-a.demo.confighub.local'

echo "==> Verifying explicit recipe manifest"
assert_contains "${manifest_file}" 'kind: Recipe'
assert_contains "${manifest_file}" 'name: global-app-us-staging'
assert_contains "${manifest_file}" "space: $(base_space)"
assert_contains "${manifest_file}" "space: $(deploy_space)"
assert_contains "${manifest_file}" 'bundleHint:'

if [[ -n "${TARGET_REF:-}" ]]; then
  echo "==> Verifying deployment has a target"
  get_unit_field "$(deploy_space)" "${DEPLOY_UNIT}" TargetID >/dev/null
fi

echo "All global-app-layers checks passed."
