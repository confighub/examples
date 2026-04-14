#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

PREFLIGHT_SCRIPT="${SCRIPT_DIR}/../preflight-live.sh"
DEFAULT_TARGET_REF="demo-flux/flux-renderer-worker-fluxoci-kubernetes-yaml-cluster"
LOCAL_FLUX_WORKLOAD_NAMESPACE="default"

target_ref="${DEFAULT_TARGET_REF}"
prefix=""
cleanup_first=0
skip_cluster_proof=0
kubeconfig_path=""
kube_context=""
apply_timeout="15m"
flux_ready_timeout_seconds=120
GENERATED_KUBECONFIG=""
CLUSTER_PROOF_REASON=""
KUBECTL_BASE=()

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./demo-flux-oci.sh [options]

Run the proven NVIDIA AICR-shaped Flux OCI lane end to end:
- preflight the target
- pick a safe short prefix for the Flux live naming budget
- materialize the layered recipe in ConfigHub
- verify the structure
- approve and apply the Flux deployment units
- print ConfigHub GUI URLs plus Flux and cluster proof surfaces

Options:
  --target <space/target>   Flux OCI target ref (default: demo-flux local lane)
  --prefix <prefix>         Prefix to use instead of auto-picking a safe short one
  --cleanup-first          Remove existing local example state before running
  --kubeconfig <path>      Optional kubeconfig for cluster proof on non-local lanes
  --kube-context <name>    Optional kubectl context name
  --skip-cluster-proof     Skip kubectl proof even if a cluster is reachable
  --apply-timeout <dur>    Timeout for cub unit apply --wait (default: 15m)
  --flux-ready-timeout <s> Seconds to wait for Flux Kustomization Ready=True (default: 120)
  -h, --help               Show this help
EOF_USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      target_ref="${2:-}"
      shift 2
      ;;
    --prefix)
      prefix="${2:-}"
      shift 2
      ;;
    --cleanup-first)
      cleanup_first=1
      shift
      ;;
    --kubeconfig)
      kubeconfig_path="${2:-}"
      shift 2
      ;;
    --kube-context)
      kube_context="${2:-}"
      shift 2
      ;;
    --skip-cluster-proof)
      skip_cluster_proof=1
      shift
      ;;
    --apply-timeout)
      apply_timeout="${2:-}"
      shift 2
      ;;
    --flux-ready-timeout)
      flux_ready_timeout_seconds="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

cleanup_temp_artifacts() {
  if [[ -n "${GENERATED_KUBECONFIG}" && -f "${GENERATED_KUBECONFIG}" ]]; then
    rm -f "${GENERATED_KUBECONFIG}"
  fi
}
trap cleanup_temp_artifacts EXIT

choose_unique_flux_demo_prefix() {
  local attempt=0
  local candidate=""
  local candidate_space=""

  while (( attempt < 32 )); do
    candidate="$(generate_safe_flux_demo_prefix)"
    candidate_space="${candidate}-${DEPLOY_FLUX_SPACE_SUFFIX}"
    if ! cub space get "${candidate_space}" >/dev/null 2>&1; then
      printf '%s\n' "${candidate}"
      return 0
    fi
    attempt=$((attempt + 1))
  done

  echo "Unable to generate a unique safe Flux demo prefix after ${attempt} attempts." >&2
  exit 1
}

run_kubectl() {
  if [[ "${#KUBECTL_BASE[@]}" -eq 0 ]]; then
    return 1
  fi

  if [[ -n "${kube_context}" ]]; then
    "${KUBECTL_BASE[@]}" --context "${kube_context}" "$@"
    return
  fi

  "${KUBECTL_BASE[@]}" "$@"
}

ensure_clean_local_state() {
  if ! state_exists; then
    return
  fi

  if [[ "${cleanup_first}" -eq 1 ]]; then
    echo "==> Cleaning up prior local example state"
    bash "${SCRIPT_DIR}/cleanup.sh"
    return
  fi

  cat >&2 <<EOF_STATE
Existing local example state was found in ${STATE_DIR}.

This helper keeps one local run active at a time.

Use either:
- ./cleanup.sh
- ./demo-flux-oci.sh --cleanup-first
EOF_STATE
  exit 2
}

cleanup_local_demo_flux_cluster_state() {
  if [[ "${cleanup_first}" -ne 1 || "${target_ref}" != "${DEFAULT_TARGET_REF}" ]]; then
    return
  fi

  if ! command -v kind >/dev/null 2>&1; then
    return
  fi

  if ! kind get clusters 2>/dev/null | grep -qx 'demo-flux'; then
    return
  fi

  local tmp_kubeconfig
  tmp_kubeconfig="$(mktemp "${TMPDIR:-/tmp}/demo-flux-cleanup-kubeconfig.XXXXXX")"
  KUBECONFIG="${tmp_kubeconfig}" kind export kubeconfig --name demo-flux >/dev/null

  echo "==> Cleaning old Flux bridge objects from the dedicated demo-flux cluster"
  KUBECONFIG="${tmp_kubeconfig}" kubectl --context kind-demo-flux delete ocirepositories,kustomizations \
    -n flux-system \
    -l app.kubernetes.io/managed-by=flux-oci-bridge \
    --ignore-not-found >/dev/null 2>&1 || true

  echo "==> Cleaning old demo workloads from the dedicated demo-flux cluster"
  KUBECONFIG="${tmp_kubeconfig}" kubectl --context kind-demo-flux delete \
    deployment/gpu-operator \
    daemonset/nvidia-device-plugin \
    service/gpu-operator \
    -n "${LOCAL_FLUX_WORKLOAD_NAMESPACE}" \
    --ignore-not-found >/dev/null 2>&1 || true

  rm -f "${tmp_kubeconfig}"
}

preflight_target() {
  local preflight_json

  echo "==> Preflighting Flux live target"
  preflight_json="$(bash "${PREFLIGHT_SCRIPT}" "${target_ref}" --json)"
  echo "${preflight_json}" | jq '{targetRef, applyReady, providerType, deliveryMode, bridgeWorker}'

  if ! jq -e '.applyReady == true' >/dev/null <<<"${preflight_json}"; then
    echo "${preflight_json}" | jq '{targetRef, reasons, nextSteps}'
    echo "Target is not ready for live delivery." >&2
    exit 2
  fi

  if ! jq -e '.deliveryMode == "flux-oci"' >/dev/null <<<"${preflight_json}"; then
    echo "${preflight_json}" | jq '{targetRef, providerType, deliveryMode, reasons, nextSteps}'
    echo "This helper is only for Flux OCI live targets." >&2
    exit 2
  fi
}

prepare_cluster_proof_lane() {
  local access_error=""

  if [[ "${skip_cluster_proof}" -eq 1 ]]; then
    CLUSTER_PROOF_REASON="cluster proof was skipped by request"
    return
  fi

  if [[ -n "${kubeconfig_path}" ]]; then
    if [[ ! -f "${kubeconfig_path}" ]]; then
      echo "Kubeconfig not found: ${kubeconfig_path}" >&2
      exit 1
    fi
    KUBECTL_BASE=(env "KUBECONFIG=${kubeconfig_path}" kubectl)
  elif [[ "${target_ref}" == demo-flux/* ]] && command -v kind >/dev/null 2>&1; then
    if kind get clusters 2>/dev/null | grep -qx 'demo-flux'; then
      GENERATED_KUBECONFIG="$(mktemp "${TMPDIR:-/tmp}/demo-flux-kubeconfig.XXXXXX")"
      KUBECONFIG="${GENERATED_KUBECONFIG}" kind export kubeconfig --name demo-flux >/dev/null
      KUBECTL_BASE=(env "KUBECONFIG=${GENERATED_KUBECONFIG}" kubectl)
      if [[ -z "${kube_context}" ]]; then
        kube_context="kind-demo-flux"
      fi
    else
      CLUSTER_PROOF_REASON="demo-flux kind cluster was not found locally"
      return
    fi
  else
    CLUSTER_PROOF_REASON="no kubeconfig was provided for cluster proof"
    return
  fi

  if access_error="$(run_kubectl get ns --request-timeout=5s -o name 2>&1 >/dev/null)"; then
    return
  fi

  KUBECTL_BASE=()
  CLUSTER_PROOF_REASON="$(printf '%s\n' "${access_error}" | sed -n '1p')"
}

approve_and_apply_flux_units() {
  local unit_csv

  unit_csv="$(deployment_unit_name gpu-operator flux),$(deployment_unit_name nvidia-device-plugin flux)"

  echo "==> Approving Flux deployment units"
  cub unit approve --space "$(flux_deploy_space)" --unit "${unit_csv}"

  echo "==> Applying Flux deployment units"
  cub unit apply --space "$(flux_deploy_space)" --unit "${unit_csv}" --wait --timeout "${apply_timeout}"
}

wait_for_flux_kustomizations_ready() {
  if [[ "${#KUBECTL_BASE[@]}" -eq 0 ]]; then
    return
  fi

  local start_seconds now_seconds elapsed_seconds
  local gpu_name device_name
  local gpu_ready device_ready

  gpu_name="$(flux_deploy_space)-$(deployment_unit_name gpu-operator flux)"
  device_name="$(flux_deploy_space)-$(deployment_unit_name nvidia-device-plugin flux)"
  start_seconds="$(date +%s)"

  echo "==> Waiting for Flux Kustomization Ready=True"

  while true; do
    gpu_ready="$(
      run_kubectl get kustomization -n flux-system "${gpu_name}" -o json 2>/dev/null \
        | jq -r '[.status.conditions[]? | select(.type == "Ready")][0].status // "Unknown"' 2>/dev/null || true
    )"
    device_ready="$(
      run_kubectl get kustomization -n flux-system "${device_name}" -o json 2>/dev/null \
        | jq -r '[.status.conditions[]? | select(.type == "Ready")][0].status // "Unknown"' 2>/dev/null || true
    )"

    if [[ "${gpu_ready}" == "True" && "${device_ready}" == "True" ]]; then
      echo "Flux Kustomizations are Ready=True."
      return
    fi

    now_seconds="$(date +%s)"
    elapsed_seconds=$((now_seconds - start_seconds))
    if (( elapsed_seconds >= flux_ready_timeout_seconds )); then
      echo "Timed out waiting for Flux Kustomization Ready=True." >&2
      echo "- ${gpu_name}: ${gpu_ready:-Unknown}" >&2
      echo "- ${device_name}: ${device_ready:-Unknown}" >&2
      return 1
    fi

    echo "- ${gpu_name}: ${gpu_ready:-Unknown}"
    echo "- ${device_name}: ${device_ready:-Unknown}"
    sleep 5
  done
}

show_confighub_proof() {
  echo "==> ConfigHub unit status"
  cub unit list --space "$(flux_deploy_space)" --quiet --json \
    | jq '.[] | {
        slug: .Unit.Slug,
        headRevision: (.Unit.HeadRevisionNum // null),
        lastAppliedRevision: (.Unit.LastAppliedRevisionNum // null),
        liveRevision: (.Unit.LiveRevisionNum // null),
        status: (.UnitStatus.Status // null),
        actionResult: (.UnitStatus.ActionResult // null)
      }'

  echo "==> GUI review URLs"
  echo "- Flux deploy space: $(gui_space_url "$(flux_deploy_space)")"
  echo "- Flux unit (gpu-operator): $(gui_unit_url "$(flux_deploy_space)" "$(deployment_unit_name gpu-operator flux)")"
  echo "- Flux unit (nvidia-device-plugin): $(gui_unit_url "$(flux_deploy_space)" "$(deployment_unit_name nvidia-device-plugin flux)")"
  echo "- Recipe manifest: $(gui_unit_url "$(recipe_space)" "${RECIPE_MANIFEST_UNIT}")"
}

show_cluster_proof() {
  if [[ "${#KUBECTL_BASE[@]}" -eq 0 ]]; then
    echo "==> Cluster proof skipped"
    echo "- reason: ${CLUSTER_PROOF_REASON:-unavailable}"
    echo "- review later:"
    echo "  kubectl get ocirepositories,kustomizations -A | grep -F '$(flux_deploy_space)'"
    echo "  kubectl get deployment/gpu-operator daemonset/nvidia-device-plugin service/gpu-operator -n ${LOCAL_FLUX_WORKLOAD_NAMESPACE}"
    return
  fi

  echo "==> Flux controller proof"
  run_kubectl get ocirepositories,kustomizations -A | grep -F "$(flux_deploy_space)" || true

  echo "==> Cluster workload proof"
  run_kubectl get deployment/gpu-operator daemonset/nvidia-device-plugin service/gpu-operator -n "${LOCAL_FLUX_WORKLOAD_NAMESPACE}"
}

require_cub
require_jq
begin_log_capture demo-flux-oci
ensure_clean_local_state
cleanup_local_demo_flux_cluster_state
preflight_target
prepare_cluster_proof_lane

if [[ -z "${prefix}" ]]; then
  prefix="$(choose_unique_flux_demo_prefix)"
fi

assert_flux_prefix_budget "${prefix}"

echo "Mode: live delivery"
echo "Using safe Flux demo prefix: ${prefix}"
echo "Flux label-value prefix budget: $(max_flux_prefix_length) characters"
echo "Target: ${target_ref}"

echo "==> Materializing layered recipe and binding Flux target"
bash "${SCRIPT_DIR}/setup.sh" "${prefix}" "${target_ref}"

echo "==> Verifying ConfigHub structure"
bash "${SCRIPT_DIR}/verify.sh"

load_state
approve_and_apply_flux_units
wait_for_flux_kustomizations_ready
show_confighub_proof
show_cluster_proof

cat <<EOF_SUMMARY
Completed live Flux OCI demo for prefix ${PREFIX}.

What this proves now:
- ConfigHub materialized the layered GPU recipe and Flux deployment units
- the Flux target passed live preflight
- the Flux deployment units were approved and applied
- GUI URLs and CLI proof surfaces are ready for review

What this does not prove:
- real NVIDIA functional runtime on GPU hardware; this example still uses stub images

Logs:
- helper: $(current_log_path demo-flux-oci)
- setup: $(current_log_path setup)
- verify: $(current_log_path verify)
EOF_SUMMARY
