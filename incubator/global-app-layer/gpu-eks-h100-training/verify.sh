#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub
require_jq
load_state
ensure_state_dir

for space in "$(base_space)" "$(platform_space)" "$(accelerator_space)" "$(os_space)" "$(recipe_space)" "$(deploy_space)"; do
  echo "==> Checking space exists: ${space}"
  cub space get "${space}" >/dev/null
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf "${tmp_dir}"' EXIT

base_file="${tmp_dir}/gpu-base.yaml"
platform_file="${tmp_dir}/gpu-platform.yaml"
accelerator_file="${tmp_dir}/gpu-accelerator.yaml"
os_file="${tmp_dir}/gpu-os.yaml"
recipe_file="${tmp_dir}/gpu-recipe.yaml"
deploy_file="${tmp_dir}/gpu-deploy.yaml"
recipe_manifest_file="${tmp_dir}/recipe-manifest.yaml"

unit_data_to_file "$(base_space)" "$(unit_name base)" "${base_file}"
unit_data_to_file "$(platform_space)" "$(unit_name platform)" "${platform_file}"
unit_data_to_file "$(accelerator_space)" "$(unit_name accelerator)" "${accelerator_file}"
unit_data_to_file "$(os_space)" "$(unit_name os)" "${os_file}"
unit_data_to_file "$(recipe_space)" "$(unit_name recipe)" "${recipe_file}"
unit_data_to_file "$(deploy_space)" "$(unit_name deployment)" "${deploy_file}"
unit_data_to_file "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}" "${recipe_manifest_file}"

echo "==> Verifying clone chain"
base_id="$(get_unit_field "$(base_space)" "$(unit_name base)" UnitID)"
platform_id="$(get_unit_field "$(platform_space)" "$(unit_name platform)" UnitID)"
accelerator_id="$(get_unit_field "$(accelerator_space)" "$(unit_name accelerator)" UnitID)"
os_id="$(get_unit_field "$(os_space)" "$(unit_name os)" UnitID)"
recipe_id="$(get_unit_field "$(recipe_space)" "$(unit_name recipe)" UnitID)"

actual="$(get_unit_field "$(platform_space)" "$(unit_name platform)" UpstreamUnitID)"
[[ "${actual}" == "${base_id}" ]] || { echo "Clone chain broken: platform upstream ${actual} != base ${base_id}" >&2; exit 1; }

actual="$(get_unit_field "$(accelerator_space)" "$(unit_name accelerator)" UpstreamUnitID)"
[[ "${actual}" == "${platform_id}" ]] || { echo "Clone chain broken: accelerator upstream ${actual} != platform ${platform_id}" >&2; exit 1; }

actual="$(get_unit_field "$(os_space)" "$(unit_name os)" UpstreamUnitID)"
[[ "${actual}" == "${accelerator_id}" ]] || { echo "Clone chain broken: os upstream ${actual} != accelerator ${accelerator_id}" >&2; exit 1; }

actual="$(get_unit_field "$(recipe_space)" "$(unit_name recipe)" UpstreamUnitID)"
[[ "${actual}" == "${os_id}" ]] || { echo "Clone chain broken: recipe upstream ${actual} != os ${os_id}" >&2; exit 1; }

actual="$(get_unit_field "$(deploy_space)" "$(unit_name deployment)" UpstreamUnitID)"
[[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: deployment upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

echo "==> Verifying staged mutations"
assert_contains "${base_file}" 'image: nvcr.io/nvidia/gpu-operator:24.6.0'
assert_contains "${base_file}" 'name: CLOUD_PROVIDER'
assert_contains "${base_file}" 'value: generic'
assert_contains "${platform_file}" 'value: eks'
assert_contains "${platform_file}" 'value: gp3'
assert_contains "${accelerator_file}" 'value: h100'
assert_contains "${accelerator_file}" 'value: nvidia-h100'
assert_contains "${os_file}" 'value: ubuntu'
assert_contains "${os_file}" 'value: 550-ubuntu22.04'
assert_contains "${recipe_file}" 'value: training'
assert_contains "${recipe_file}" 'value: training-smoke'
assert_contains "${deploy_file}" 'namespace: cluster-a'
assert_contains "${deploy_file}" 'value: cluster-a'

echo "==> Verifying explicit GPU recipe manifest"
assert_contains "${recipe_manifest_file}" 'kind: Recipe'
assert_contains "${recipe_manifest_file}" 'component: gpu-operator'
assert_contains "${recipe_manifest_file}" 'platform: eks'
assert_contains "${recipe_manifest_file}" 'accelerator: h100'
assert_contains "${recipe_manifest_file}" 'os: ubuntu'
assert_contains "${recipe_manifest_file}" 'intent: training'
assert_contains "${recipe_manifest_file}" "space: $(base_space)"
assert_contains "${recipe_manifest_file}" "space: $(deploy_space)"
assert_contains "${recipe_manifest_file}" 'bundleHint:'

if [[ -n "${TARGET_REF:-}" ]]; then
  echo "==> Verifying deployment unit has a target"
  get_unit_field "$(deploy_space)" "$(unit_name deployment)" TargetID >/dev/null
fi

echo "All global-app-layer gpu-eks-h100-training checks passed."
