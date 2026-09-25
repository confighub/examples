#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

json_mode=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --json)
      json_mode=true
      shift
      ;;
    -h|--help)
      verify_usage
      exit 0
      ;;
    *)
      echo "Unknown flag: $1" >&2
      verify_usage >&2
      exit 1
      ;;
  esac
done

verify_impl() {
  require_cub
  require_jq
  begin_log_capture verify
  load_state
  ensure_state_dir

  for space in "$(base_space)" "$(platform_space)" "$(accelerator_space)" "$(profile_space)" "$(recipe_space)" "$(deploy_space)" "$(flux_deploy_space)" "$(argo_deploy_space)"; do
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
    profile_file="${tmp_dir}/${component}-profile.yaml"
    recipe_file="${tmp_dir}/${component}-recipe.yaml"
    direct_deploy_file="${tmp_dir}/${component}-deploy-direct.yaml"
    flux_deploy_file="${tmp_dir}/${component}-deploy-flux.yaml"
    argo_deploy_file="${tmp_dir}/${component}-deploy-argo.yaml"

    unit_data_to_file "$(base_space)" "$(unit_name "${component}" base)" "${base_file}"
    unit_data_to_file "$(platform_space)" "$(unit_name "${component}" platform)" "${platform_file}"
    unit_data_to_file "$(accelerator_space)" "$(unit_name "${component}" accelerator)" "${accelerator_file}"
    unit_data_to_file "$(profile_space)" "$(unit_name "${component}" profile)" "${profile_file}"
    unit_data_to_file "$(recipe_space)" "$(unit_name "${component}" recipe)" "${recipe_file}"
    unit_data_to_file "$(deploy_space)" "$(deployment_unit_name "${component}" direct)" "${direct_deploy_file}"
    unit_data_to_file "$(flux_deploy_space)" "$(deployment_unit_name "${component}" flux)" "${flux_deploy_file}"
    unit_data_to_file "$(argo_deploy_space)" "$(deployment_unit_name "${component}" argo)" "${argo_deploy_file}"

    base_id="$(get_unit_field "$(base_space)" "$(unit_name "${component}" base)" UnitID)"
    platform_id="$(get_unit_field "$(platform_space)" "$(unit_name "${component}" platform)" UnitID)"
    accelerator_id="$(get_unit_field "$(accelerator_space)" "$(unit_name "${component}" accelerator)" UnitID)"
    profile_id="$(get_unit_field "$(profile_space)" "$(unit_name "${component}" profile)" UnitID)"
    recipe_id="$(get_unit_field "$(recipe_space)" "$(unit_name "${component}" recipe)" UnitID)"

    actual="$(get_unit_field "$(platform_space)" "$(unit_name "${component}" platform)" UpstreamUnitID)"
    [[ "${actual}" == "${base_id}" ]] || { echo "Clone chain broken: ${component} platform upstream ${actual} != base ${base_id}" >&2; exit 1; }

    actual="$(get_unit_field "$(accelerator_space)" "$(unit_name "${component}" accelerator)" UpstreamUnitID)"
    [[ "${actual}" == "${platform_id}" ]] || { echo "Clone chain broken: ${component} accelerator upstream ${actual} != platform ${platform_id}" >&2; exit 1; }

    actual="$(get_unit_field "$(profile_space)" "$(unit_name "${component}" profile)" UpstreamUnitID)"
    [[ "${actual}" == "${accelerator_id}" ]] || { echo "Clone chain broken: ${component} profile upstream ${actual} != accelerator ${accelerator_id}" >&2; exit 1; }

    actual="$(get_unit_field "$(recipe_space)" "$(unit_name "${component}" recipe)" UpstreamUnitID)"
    [[ "${actual}" == "${profile_id}" ]] || { echo "Clone chain broken: ${component} recipe upstream ${actual} != profile ${profile_id}" >&2; exit 1; }

    actual="$(get_unit_field "$(deploy_space)" "$(deployment_unit_name "${component}" direct)" UpstreamUnitID)"
    [[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: ${component} direct deployment upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

    actual="$(get_unit_field "$(flux_deploy_space)" "$(deployment_unit_name "${component}" flux)" UpstreamUnitID)"
    [[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: ${component} flux deployment upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

    actual="$(get_unit_field "$(argo_deploy_space)" "$(deployment_unit_name "${component}" argo)" UpstreamUnitID)"
    [[ "${actual}" == "${recipe_id}" ]] || { echo "Clone chain broken: ${component} argo deployment upstream ${actual} != recipe ${recipe_id}" >&2; exit 1; }

    case "${component}" in
      rag-server)
        assert_contains "${platform_file}" 'value: gp3'
        assert_contains "${platform_file}" 'value: alb'
        assert_contains "${accelerator_file}" "value: ${ACCELERATOR_VALUE}"
        assert_contains "${profile_file}" "value: ${STACK}"
        assert_contains "${profile_file}" "value: ${PROFILE_VALUE}"
        assert_contains "${recipe_file}" "value: ${RECIPE_VALUE}"
        assert_contains "${recipe_file}" 'value: enterprise-default'
        assert_contains "${direct_deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
        assert_contains "${direct_deploy_file}" "value: ${DEPLOY_NAMESPACE}"
        assert_contains "${direct_deploy_file}" "value: ${DEPLOY_REGION}"
        if [[ "${STACK}" == "ollama" ]]; then
          assert_contains "${direct_deploy_file}" "host.docker.internal"
        fi
        ;;
      nim-llm)
        assert_contains "${platform_file}" 'value: gp3'
        assert_contains "${accelerator_file}" "value: ${ACCELERATOR_VALUE}"
        assert_contains "${accelerator_file}" 'value: nvidia-h100'
        assert_contains "${profile_file}" "value: ${STACK}"
        assert_contains "${profile_file}" "value: ${PROFILE_VALUE}"
        assert_contains "${recipe_file}" "value: ${RECIPE_VALUE}"
        assert_contains "${direct_deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
        ;;
      nim-embedding)
        assert_contains "${platform_file}" 'value: gp3'
        assert_contains "${accelerator_file}" "value: ${ACCELERATOR_VALUE}"
        assert_contains "${accelerator_file}" 'value: nvidia-h100'
        assert_contains "${profile_file}" "value: ${STACK}"
        assert_contains "${profile_file}" "value: ${PROFILE_VALUE}"
        assert_contains "${recipe_file}" "value: ${RECIPE_VALUE}"
        assert_contains "${direct_deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
        ;;
      vector-db)
        assert_contains "${platform_file}" 'value: gp3'
        assert_contains "${accelerator_file}" "value: ${ACCELERATOR_VALUE}"
        assert_contains "${profile_file}" "value: ${STACK}"
        assert_contains "${profile_file}" "value: HNSW"
        assert_contains "${recipe_file}" "value: ${RECIPE_VALUE}"
        assert_contains "${direct_deploy_file}" "namespace: ${DEPLOY_NAMESPACE}"
        ;;
    esac
  done

  echo "==> Verifying explicit recipe manifest"
  assert_contains "${recipe_manifest_file}" 'kind: Recipe'
  assert_contains "${recipe_manifest_file}" 'app: enterprise-rag'
  assert_contains "${recipe_manifest_file}" 'name: rag-server'
  assert_contains "${recipe_manifest_file}" 'name: nim-llm'
  assert_contains "${recipe_manifest_file}" 'name: nim-embedding'
  assert_contains "${recipe_manifest_file}" 'name: vector-db'
  assert_contains "${recipe_manifest_file}" 'platform: kgpu'
  assert_contains "${recipe_manifest_file}" 'accelerator: h100'
  assert_contains "${recipe_manifest_file}" 'profile: medium'
  assert_contains "${recipe_manifest_file}" 'recipe: enterprise-rag'
  assert_contains "${recipe_manifest_file}" "stack: ${STACK}"
  assert_contains "${recipe_manifest_file}" "space: $(base_space)"
  assert_contains "${recipe_manifest_file}" "space: $(deploy_space)"
  assert_contains "${recipe_manifest_file}" "space: $(flux_deploy_space)"
  assert_contains "${recipe_manifest_file}" "space: $(argo_deploy_space)"
  assert_contains "${recipe_manifest_file}" 'name: direct'
  assert_contains "${recipe_manifest_file}" 'name: flux'
  assert_contains "${recipe_manifest_file}" 'name: argo'
  assert_contains "${recipe_manifest_file}" 'bundleHint:'

  if [[ -n "${DIRECT_TARGET_REF:-}" ]]; then
    echo "==> Verifying direct deployment units have a target"
    for component in "${COMPONENTS[@]}"; do
      get_unit_field "$(deploy_space)" "$(deployment_unit_name "${component}" direct)" TargetID >/dev/null
    done
  fi

  if [[ -n "${FLUX_TARGET_REF:-}" ]]; then
    echo "==> Verifying Flux deployment units have a target"
    for component in "${COMPONENTS[@]}"; do
      get_unit_field "$(flux_deploy_space)" "$(deployment_unit_name "${component}" flux)" TargetID >/dev/null
    done
  fi

  if [[ -n "${ARGO_TARGET_REF:-}" ]]; then
    echo "==> Verifying Argo deployment units have a target"
    for component in "${COMPONENTS[@]}"; do
      get_unit_field "$(argo_deploy_space)" "$(deployment_unit_name "${component}" argo)" TargetID >/dev/null
    done
  fi

  echo "All enterprise-rag-blueprint checks passed (stack=${STACK})."
}

if [[ "${json_mode}" != true ]]; then
  verify_impl
  exit 0
fi

require_jq

if verify_output="$(verify_impl 2>&1)"; then
  load_state
  jq -n \
    --arg example "${EXAMPLE_NAME}" \
    --arg prefix "${PREFIX}" \
    --arg stack "${STACK}" \
    --arg summary "All enterprise-rag-blueprint checks passed (stack=${STACK})." \
    --argjson targetExpected "$(if [[ -n "${DIRECT_TARGET_REF:-}" || -n "${FLUX_TARGET_REF:-}" || -n "${ARGO_TARGET_REF:-}" || -n "${TARGET_REF:-}" ]]; then echo true; else echo false; fi)" \
    --argjson spacesChecked "$(all_spaces | jq -Rsc 'split("\n")[:-1]')" \
    --argjson unitsChecked "$(all_unit_refs | jq -Rsc 'split("\n")[:-1]')" \
    --argjson checks '[
      "spaces-exist",
      "clone-chains",
      "component-mutations",
      "recipe-manifest",
      "target-bindings-when-present"
    ]' \
    '{
      ok: true,
      example: $example,
      prefix: $prefix,
      stack: $stack,
      targetExpected: $targetExpected,
      spacesChecked: $spacesChecked,
      unitsChecked: $unitsChecked,
      checks: $checks,
      summary: $summary
    }'
else
  jq -n \
    --arg example "${EXAMPLE_NAME}" \
    --arg error "${verify_output}" \
    '{
      ok: false,
      example: $example,
      error: ($error | gsub("\\n+$"; ""))
    }'
  exit 1
fi
