#!/usr/bin/env bash
# Shared e2e helpers for the global-app-layer package.
# Source this from any script under incubator/global-app-layer/e2e.

E2E_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LAYER_DIR="$(dirname "${E2E_DIR}")"
REPO_ROOT="$(cd "${LAYER_DIR}/../../.." && pwd)"
GITOPS_IMPORT_DIR="${REPO_ROOT}/gitops-import"
KUBECONFIG_PATH="${GITOPS_IMPORT_DIR}/var/gitops-import.kubeconfig"

# Infrastructure defaults — override via env if your cluster is different.
WORKER_SPACE="${WORKER_SPACE:-gitops-import-test}"
K8S_TARGET="${K8S_TARGET:-worker-kubernetes-yaml-cluster}"
ARGO_TARGET="${ARGO_TARGET:-worker-argocdrenderer-kubernetes-yaml-cluster}"
ARGOCD_PORT="${ARGOCD_PORT:-9080}"
GITOPS_STAGE_DIR="${E2E_DIR}/.gitops-stage"

require_kubeconfig() {
  if [[ ! -f "${KUBECONFIG_PATH}" ]]; then
    echo "No kubeconfig at ${KUBECONFIG_PATH}" >&2
    echo "Run gitops-import/bin/create-cluster + install-argocd + install-worker first." >&2
    exit 1
  fi
  export KUBECONFIG="${KUBECONFIG_PATH}"
}

require_infrastructure() {
  require_kubeconfig

  if ! kubectl cluster-info >/dev/null 2>&1; then
    echo "Cluster not reachable. Is Docker Desktop running? Is the kind cluster up?" >&2
    exit 1
  fi

  local worker_condition
  worker_condition="$(cub worker get worker --space "${WORKER_SPACE}" --json 2>/dev/null \
    | jq -r '.BridgeWorker.Condition' 2>/dev/null)" || true
  if [[ "${worker_condition}" != "Ready" ]]; then
    echo "Worker not Ready (condition=${worker_condition:-unknown})." >&2
    echo "Run: CUB_SPACE=${WORKER_SPACE} gitops-import/bin/install-worker" >&2
    exit 1
  fi
}

# target_ref for direct apply
direct_target() {
  echo "${WORKER_SPACE}/${K8S_TARGET}"
}

# target_ref for ArgoCD apply
argo_target() {
  echo "${WORKER_SPACE}/${ARGO_TARGET}"
}

require_example() {
  local example_name="$1"
  local example_dir="${LAYER_DIR}/${example_name}"
  if [[ ! -d "${example_dir}" ]]; then
    echo "Error: Example directory not found: ${example_dir}" >&2
    exit 1
  fi
  if [[ ! -f "${example_dir}/.state/state.env" ]]; then
    echo "Error: No state file in ${example_dir}. Run ./setup.sh first." >&2
    exit 1
  fi
}

# Source the example's lib.sh and load its state.
load_example() {
  local example_name="$1"
  local example_dir="${LAYER_DIR}/${example_name}"
  # shellcheck disable=SC1090
  source "${example_dir}/lib.sh"
  load_state
}

# Return the deploy space name for the loaded example.
example_deploy_space() {
  deploy_space
}

# Return deploy namespace.
example_deploy_namespace() {
  echo "${DEPLOY_NAMESPACE}"
}

# Return the list of deployment unit names for the loaded example.
example_deploy_units() {
  local component
  for component in "${COMPONENTS[@]}"; do
    unit_name "${component}" deployment
  done
}

# Return component names for the loaded example.
example_components() {
  local component
  for component in "${COMPONENTS[@]}"; do
    echo "${component}"
  done
}

# Create a namespace if it doesn't exist.
ensure_namespace() {
  local ns="$1"
  if ! kubectl get namespace "${ns}" >/dev/null 2>&1; then
    kubectl create namespace "${ns}"
  fi
}

# Wait for a deployment to have at least 1 ready replica.
wait_for_deployment() {
  local ns="$1"
  local name="$2"
  local timeout="${3:-60s}"
  echo "  Waiting for deployment ${ns}/${name} (timeout ${timeout})..."
  kubectl wait --for=condition=Available --timeout="${timeout}" \
    -n "${ns}" "deployment/${name}" 2>/dev/null
}

# Wait for a statefulset to have at least 1 ready replica.
wait_for_statefulset() {
  local ns="$1"
  local name="$2"
  local timeout="${3:-90s}"
  echo "  Waiting for statefulset ${ns}/${name} (timeout ${timeout})..."
  kubectl rollout status statefulset/"${name}" -n "${ns}" --timeout="${timeout}" 2>/dev/null
}

# Wait for a daemonset to have at least 1 ready pod.
wait_for_daemonset() {
  local ns="$1"
  local name="$2"
  local timeout="${3:-60s}"
  echo "  Waiting for daemonset ${ns}/${name} (timeout ${timeout})..."
  kubectl rollout status daemonset/"${name}" -n "${ns}" --timeout="${timeout}" 2>/dev/null
}

# Assert a resource exists in the cluster.
assert_resource_exists() {
  local ns="$1"
  local kind="$2"
  local name="$3"
  if kubectl get "${kind}" "${name}" -n "${ns}" >/dev/null 2>&1; then
    echo "  OK: ${kind}/${name} exists in ${ns}"
  else
    echo "  FAIL: ${kind}/${name} not found in ${ns}" >&2
    return 1
  fi
}

# Assert a resource does NOT exist.
assert_resource_gone() {
  local ns="$1"
  local kind="$2"
  local name="$3"
  if kubectl get "${kind}" "${name}" -n "${ns}" >/dev/null 2>&1; then
    echo "  FAIL: ${kind}/${name} still exists in ${ns}" >&2
    return 1
  else
    echo "  OK: ${kind}/${name} gone from ${ns}"
  fi
}

# Assert a field value in a resource.
assert_resource_field() {
  local ns="$1"
  local kind="$2"
  local name="$3"
  local jsonpath="$4"
  local expected="$5"
  local actual
  actual="$(kubectl get "${kind}" "${name}" -n "${ns}" -o jsonpath="${jsonpath}" 2>/dev/null)" || {
    echo "  FAIL: cannot read ${kind}/${name} in ${ns}" >&2
    return 1
  }
  if [[ "${actual}" == "${expected}" ]]; then
    echo "  OK: ${kind}/${name} ${jsonpath} = ${expected}"
  else
    echo "  FAIL: ${kind}/${name} ${jsonpath} = '${actual}', expected '${expected}'" >&2
    return 1
  fi
}

# Assert a ConfigHub unit exists.
assert_unit_exists() {
  local space="$1"
  local unit="$2"
  if cub unit get --space "${space}" "${unit}" >/dev/null 2>&1; then
    echo "  OK: unit ${space}/${unit} exists"
  else
    echo "  FAIL: unit ${space}/${unit} not found" >&2
    return 1
  fi
}

# Clean all units and links from a space (without deleting the space itself).
clean_space_contents() {
  local space="$1"
  local link_ids
  link_ids="$(cub link list --space "${space}" --json 2>/dev/null \
    | jq -r '.[].Link.LinkID' 2>/dev/null)" || true
  if [[ -n "${link_ids}" ]]; then
    while IFS= read -r lid; do
      [[ -n "${lid}" ]] && cub link delete --space "${space}" "${lid}" 2>/dev/null || true
    done <<< "${link_ids}"
  fi

  local unit_slugs
  unit_slugs="$(cub unit list --space "${space}" --json 2>/dev/null \
    | jq -r '.[].Unit.Slug' 2>/dev/null)" || true
  if [[ -n "${unit_slugs}" ]]; then
    while IFS= read -r u; do
      [[ -n "${u}" ]] && cub unit delete --space "${space}" "${u}" --force 2>/dev/null || true
    done <<< "${unit_slugs}"
  fi
}

# Get unit data to stdout.
unit_data() {
  local space="$1"
  local unit="$2"
  cub unit get --space "${space}" --data-only "${unit}"
}

section() {
  echo ""
  echo "================================================================"
  echo "  $*"
  echo "================================================================"
  echo ""
}

step() {
  echo "==> $*"
}
