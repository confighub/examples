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
recipe_manifest_file="${tmp_dir}/recipe-manifest.yaml"
unit_data_to_file "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}" "${recipe_manifest_file}"

echo "==> Verifying clone chains"
for component in "${COMPONENTS[@]}"; do
  base_file="${tmp_dir}/${component}-base.yaml"
  platform_file="${tmp_dir}/${component}-platform.yaml"
  accelerator_file="${tmp_dir}/${component}-accelerator.yaml"
  os_file="${tmp_dir}/${component}-os.yaml"
  recipe_file="${tmp_dir}/${component}-recipe.yaml"
  deploy_file="${tmp_dir}/${component}-deploy.yaml"

  unit_data_to_file "$(base_space)" "$(unit_name "${component}" base)" "${base_file}"
  unit_data_to_file "$(platform_space)" "$(unit_name "${component}" platform)" "${platform_file}"
  unit_data_to_file "$(accelerator_space)" "$(unit_name "${component}" accelerator)" "${accelerator_file}"
  unit_data_to_file "$(os_space)" "$(unit_name "${component}" os)" "${os_file}"
  unit_data_to_file "$(recipe_space)" "$(unit_name "${component}" recipe)" "${recipe_file}"
  unit_data_to_file "$(deploy_space)" "$(unit_name "${component}" deployment)" "${deploy_file}"

  base_id="$(get_unit_field "$(base_space)" "$(unit_name "${component}" base)" UnitID)"
  platform_id="$(get_unit_field "$(platform_space)" "$(unit_name "${component}" platform)" UnitID)"
  accelerator_id="$(get_unit_field "$(accelerator_space)" "$(unit_name "${component}" accelerator)" UnitID)"
  os_id="$(get_unit_field "$(os_space)" "$(unit_name "${component}" os)" UnitID)"
  recipe_id="$(get_unit_field "$(recipe_space)" "$(unit_name "${component}" recipe)" UnitID)"

  actual="$(get_unit_field "$(platform_space)" "$(unit_name "${component}" platform)" UpstreamUnitID)"
  [[ "${actual}" == "${base_id}" ]] || { echo "Clone chain broken: ${component} platform upstream ${actual} != base ${base_id}" >&2; exit 1; }

  actual="$(get_unit_field "$(accelerator_space)" "$(unit_name "${component}" accelerator)" UpstreamUnitID)"
  [[ "${actual}" == "${platform_id}" ]] || { echo "Clone chain broken: ${component} accelerator upstream ${actual} != platform ${platform_id}" >&2; exit 1; }

  actual="$(get_unit_field "$(os_space)" "$(unit_name "${component}" os)" UpstreamUnitID)"
  [[ "${actual}" == "${accelerator_id}" ]] || { echo "Clone chain broken: ${component} os upstream ${actual} != accelerator ${accelerator_id}" >&2; exit 1; }

  actual="$(get_unit_field "$(recipe_space)" "$(unit_name "${component}" recipe)" UpstreamUnitID)"
  [[ "${actual}" == "${os_id}" ]] || { echo "Clone chain broken: ${component} recipe upstream ${actual} != os ${os_id}" >&2; exit 1; }

  actual="$(get_unit_field "$(deploy_space)" "$(unit_name "${component}" deployment)" UpstreamUnitID)"
  [[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: ${component} deployment upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

  case "${component}" in
    gpu-operator)
      # Stub image: nginx:1.27-alpine (replace with nvcr.io/nvidia/gpu-operator on real GPU clusters)
      assert_contains "${base_file}" 'image: nginx:1.27-alpine'
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
      ;;
    nvidia-device-plugin)
      # Stub image: busybox:1.37 (replace with nvcr.io/nvidia/k8s-device-plugin on real GPU clusters)
      assert_contains "${base_file}" 'image: busybox:1.37'
      assert_contains "${base_file}" 'value: generic'
      assert_contains "${platform_file}" 'value: eks'
      assert_contains "${platform_file}" 'value: eks-gp3'
      assert_contains "${accelerator_file}" 'value: h100'
      assert_contains "${accelerator_file}" 'value: nvidia-h100'
      assert_contains "${os_file}" 'value: ubuntu'
      assert_contains "${os_file}" 'value: ubuntu-h100'
      assert_contains "${recipe_file}" 'value: training'
      assert_contains "${recipe_file}" 'value: training-smoke'
      assert_contains "${deploy_file}" 'namespace: cluster-a'
      assert_contains "${deploy_file}" 'value: cluster-a'
      ;;
  esac
done

echo "==> Verifying explicit GPU recipe manifest"
assert_contains "${recipe_manifest_file}" 'kind: Recipe'
assert_contains "${recipe_manifest_file}" 'app: gpu-platform'
assert_contains "${recipe_manifest_file}" 'name: gpu-operator'
assert_contains "${recipe_manifest_file}" 'name: nvidia-device-plugin'
assert_contains "${recipe_manifest_file}" 'platform: eks'
assert_contains "${recipe_manifest_file}" 'accelerator: h100'
assert_contains "${recipe_manifest_file}" 'os: ubuntu'
assert_contains "${recipe_manifest_file}" 'intent: training'
assert_contains "${recipe_manifest_file}" "space: $(base_space)"
assert_contains "${recipe_manifest_file}" "space: $(deploy_space)"
assert_contains "${recipe_manifest_file}" 'bundleHint:'

if [[ -n "${TARGET_REF:-}" ]]; then
  echo "==> Verifying deployment units have a target"
  get_unit_field "$(deploy_space)" "$(unit_name gpu-operator deployment)" TargetID >/dev/null
  get_unit_field "$(deploy_space)" "$(unit_name nvidia-device-plugin deployment)" TargetID >/dev/null
fi

echo "All global-app-layer gpu-eks-h100-training checks passed."
