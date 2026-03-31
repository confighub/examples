#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"

script_checks=(
  "${repo_root}/scripts/verify.sh"
  "${repo_root}/spring-platform/springboot-platform-app-centric/setup.sh"
  "${repo_root}/spring-platform/springboot-platform-app-centric/verify.sh"
  "${repo_root}/spring-platform/springboot-platform-app-centric/cleanup.sh"
  "${repo_root}/spring-platform/springboot-platform-app-centric/demo.sh"
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
  "${repo_root}/incubator/global-app-layer/find-runs.sh"
  "${repo_root}/incubator/global-app-layer/preflight-live.sh"
  "${repo_root}/incubator/global-app-layer/e2e/lib.sh"
  "${repo_root}/incubator/global-app-layer/e2e/01-brownfield.sh"
  "${repo_root}/incubator/global-app-layer/e2e/02-greenfield.sh"
  "${repo_root}/incubator/global-app-layer/e2e/03-bridge.sh"
  "${repo_root}/incubator/global-app-layer/e2e/deliver-direct.sh"
  "${repo_root}/incubator/global-app-layer/e2e/deliver-argo.sh"
  "${repo_root}/incubator/global-app-layer/e2e/assert-cluster.sh"
  "${repo_root}/incubator/global-app-layer/e2e/run-all.sh"
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

# ==================== AI Guide Standard Checks ====================
#
# These checks verify that important examples follow the AI-first demo
# pacing standard. Incubator examples require contracts.md and --explain
# support. Stable examples have lighter requirements.

ai_guide_examples=(
  # stable examples
  "${repo_root}/spring-platform/springboot-platform-app-centric"
  "${repo_root}/spring-platform/springboot-platform-app"
  "${repo_root}/campaigns-demo"
  "${repo_root}/promotion-demo-data"
  # global-app-layer examples
  "${repo_root}/incubator/global-app-layer/single-component"
  "${repo_root}/incubator/global-app-layer/frontend-postgres"
  "${repo_root}/incubator/global-app-layer/realistic-app"
  "${repo_root}/incubator/global-app-layer/gpu-eks-h100-training"
  "${repo_root}/incubator/global-app-layer/bundle-evidence-sample"
  # gitops import examples
  "${repo_root}/incubator/gitops-import-argo"
  "${repo_root}/incubator/gitops-import-flux"
  # mutation examples
  "${repo_root}/incubator/platform-write-api"
  # apptique examples
  "${repo_root}/incubator/apptique-argo-app-of-apps"
  "${repo_root}/incubator/apptique-argo-applicationset"
  "${repo_root}/incubator/apptique-flux-monorepo"
  # discovery and evidence examples
  "${repo_root}/incubator/artifact-workflow"
  "${repo_root}/incubator/combined-git-live"
  "${repo_root}/incubator/connect-and-compare"
  "${repo_root}/incubator/connected-summary-storage"
  "${repo_root}/incubator/custom-ownership-detectors"
  "${repo_root}/incubator/demo-data-adt"
  "${repo_root}/incubator/fleet-import"
  "${repo_root}/incubator/flux-boutique"
  "${repo_root}/incubator/graph-export"
  "${repo_root}/incubator/import-from-bundle"
  "${repo_root}/incubator/import-from-live"
  "${repo_root}/incubator/lifecycle-hazards"
  "${repo_root}/incubator/orphans"
  "${repo_root}/incubator/platform-example"
  "${repo_root}/incubator/watch-webhook"
)

# Examples intentionally exempt from contracts.md requirement
exempt_from_contracts=(
  "${repo_root}/incubator/watch-webhook"  # lightweight event example
  "${repo_root}/campaigns-demo"           # stable demo data
  "${repo_root}/promotion-demo-data"      # stable demo data
)

# Examples exempt from setup.sh --explain requirement
exempt_from_explain=(
  "${repo_root}/campaigns-demo"           # stable demo data
  "${repo_root}/promotion-demo-data"      # stable demo data
)

for example_dir in "${ai_guide_examples[@]}"; do
  example_name="${example_dir##*/}"
  echo "==> Checking AI guide standard: ${example_name}"

  # Check README.md exists
  if [[ ! -f "${example_dir}/README.md" ]]; then
    echo "FAIL: ${example_name} missing README.md" >&2
    exit 1
  fi

  # Check AI_START_HERE.md exists
  ai_guide="${example_dir}/AI_START_HERE.md"
  if [[ ! -f "${ai_guide}" ]]; then
    echo "FAIL: ${example_name} missing AI_START_HERE.md" >&2
    exit 1
  fi

  # Check contracts.md exists (unless exempt)
  is_exempt=false
  for exempt in "${exempt_from_contracts[@]}"; do
    if [[ "${example_dir}" == "${exempt}" ]]; then
      is_exempt=true
      break
    fi
  done
  if [[ "${is_exempt}" == "false" && ! -f "${example_dir}/contracts.md" ]]; then
    echo "FAIL: ${example_name} missing contracts.md" >&2
    exit 1
  fi

  # Check contracts.md contains required markers (unless exempt)
  if [[ "${is_exempt}" == "false" && -f "${example_dir}/contracts.md" ]]; then
    if ! grep -qi 'mutates:' "${example_dir}/contracts.md"; then
      echo "FAIL: ${example_name}/contracts.md missing 'mutates:' markers" >&2
      exit 1
    fi
    if ! grep -qi 'proves:' "${example_dir}/contracts.md"; then
      echo "FAIL: ${example_name}/contracts.md missing 'proves:' markers" >&2
      exit 1
    fi
  fi

  # Check setup.sh supports --explain (unless exempt)
  is_explain_exempt=false
  for exempt in "${exempt_from_explain[@]}"; do
    if [[ "${example_dir}" == "${exempt}" ]]; then
      is_explain_exempt=true
      break
    fi
  done
  if [[ "${is_explain_exempt}" == "false" && -f "${example_dir}/setup.sh" ]]; then
    if ! grep -q '\-\-explain' "${example_dir}/setup.sh"; then
      echo "FAIL: ${example_name}/setup.sh does not support --explain" >&2
      exit 1
    fi
    # Check setup.sh supports --explain-json
    if ! grep -q '\-\-explain-json' "${example_dir}/setup.sh"; then
      echo "FAIL: ${example_name}/setup.sh does not support --explain-json" >&2
      exit 1
    fi
  fi

  # Check AI guide contains ## CRITICAL: Demo Pacing (case-insensitive)
  if ! grep -qi '## CRITICAL.*Demo.*Pacing' "${ai_guide}"; then
    echo "FAIL: ${example_name}/AI_START_HERE.md missing '## CRITICAL: Demo Pacing'" >&2
    exit 1
  fi

  # Check AI guide contains ## Suggested Prompt
  if ! grep -qi '## Suggested Prompt' "${ai_guide}"; then
    echo "FAIL: ${example_name}/AI_START_HERE.md missing '## Suggested Prompt'" >&2
    exit 1
  fi

  # Check AI guide contains at least one Stage heading
  if ! grep -qE '## Stage [0-9]|### Stage [0-9]' "${ai_guide}"; then
    echo "FAIL: ${example_name}/AI_START_HERE.md missing Stage headings" >&2
    exit 1
  fi

  # Check AI guide contains GUI gap:
  if ! grep -q 'GUI gap:' "${ai_guide}"; then
    echo "FAIL: ${example_name}/AI_START_HERE.md missing 'GUI gap:'" >&2
    exit 1
  fi

  # Check AI guide contains GUI feature ask: (or GUI ask:)
  if ! grep -qE 'GUI (feature )?ask:' "${ai_guide}"; then
    echo "FAIL: ${example_name}/AI_START_HERE.md missing 'GUI feature ask:' or 'GUI ask:'" >&2
    exit 1
  fi

  echo "    PASS: ${example_name}"
done

echo "All example checks passed."
