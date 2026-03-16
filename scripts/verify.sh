#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

script_checks=(
  "${repo_root}/scripts/verify.sh"
  "${repo_root}/incubator/global-app-layer/single-component/lib.sh"
  "${repo_root}/incubator/global-app-layer/single-component/setup.sh"
  "${repo_root}/incubator/global-app-layer/single-component/set-target.sh"
  "${repo_root}/incubator/global-app-layer/single-component/upgrade-chain.sh"
  "${repo_root}/incubator/global-app-layer/single-component/verify.sh"
  "${repo_root}/incubator/global-app-layer/single-component/cleanup.sh"
  "${repo_root}/incubator/global-app-layer/frontend-postgres/lib.sh"
  "${repo_root}/incubator/global-app-layer/frontend-postgres/setup.sh"
  "${repo_root}/incubator/global-app-layer/frontend-postgres/set-target.sh"
  "${repo_root}/incubator/global-app-layer/frontend-postgres/upgrade-chain.sh"
  "${repo_root}/incubator/global-app-layer/frontend-postgres/verify.sh"
  "${repo_root}/incubator/global-app-layer/frontend-postgres/cleanup.sh"
  "${repo_root}/incubator/global-app-layer/realistic-app/lib.sh"
  "${repo_root}/incubator/global-app-layer/realistic-app/setup.sh"
  "${repo_root}/incubator/global-app-layer/realistic-app/set-target.sh"
  "${repo_root}/incubator/global-app-layer/realistic-app/upgrade-chain.sh"
  "${repo_root}/incubator/global-app-layer/realistic-app/verify.sh"
  "${repo_root}/incubator/global-app-layer/realistic-app/cleanup.sh"
  "${repo_root}/incubator/global-app-layer/gpu-eks-h100-training/lib.sh"
  "${repo_root}/incubator/global-app-layer/gpu-eks-h100-training/setup.sh"
  "${repo_root}/incubator/global-app-layer/gpu-eks-h100-training/set-target.sh"
  "${repo_root}/incubator/global-app-layer/gpu-eks-h100-training/upgrade-chain.sh"
  "${repo_root}/incubator/global-app-layer/gpu-eks-h100-training/verify.sh"
  "${repo_root}/incubator/global-app-layer/gpu-eks-h100-training/cleanup.sh"
)

for script_path in "${script_checks[@]}"; do
  if [[ ! -f "${script_path}" ]]; then
    echo "Missing required script: ${script_path}" >&2
    exit 1
  fi
  echo "==> Linting shell script: ${script_path##*/}"
  bash -n "${script_path}"
done

bundle_roots=(
  "${repo_root}/incubator/cub-run-fixtures"
)

for bundle_root in "${bundle_roots[@]}"; do
  if [[ ! -d "${bundle_root}" ]]; then
    echo "==> Skipping missing bundle root: ${bundle_root##*/}"
    continue
  fi
  while IFS= read -r bundle_dir; do
    [[ -d "${bundle_dir}" ]] || continue
    bundle_name="$(basename "${bundle_dir}")"
    if [[ ! -f "${bundle_dir}/up.yaml" && ! -f "${bundle_dir}/up.yml" ]]; then
      echo "Bundle ${bundle_name} under ${bundle_root} is missing up.yaml/up.yml" >&2
      exit 1
    fi
    manifest_count="$(find "${bundle_dir}" -maxdepth 1 -type f \( -name '*.yaml' -o -name '*.yml' \) ! -name 'up.yaml' ! -name 'up.yml' | wc -l | tr -d ' ')"
    if [[ "${manifest_count}" -eq 0 ]]; then
      echo "Bundle ${bundle_name} under ${bundle_root} has no manifest files besides up.yaml" >&2
      exit 1
    fi
    echo "==> Verified bundle layout: ${bundle_root##*/}/${bundle_name}"
  done < <(find "${bundle_root}" -mindepth 1 -maxdepth 1 -type d | sort)
done

echo "All example checks passed."
