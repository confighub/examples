#!/usr/bin/env bash
# E2E: Greenfield flow
#
# Proves: base YAML -> ConfigHub layered chain -> cub unit apply -> resources on cluster.
#
# Runs each global-app-layer example through: setup -> verify -> apply -> cluster check -> cleanup.
#
# Prereqs: gitops-import infrastructure (kind + ArgoCD + worker).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"

EXAMPLES_DIR="${REPO_ROOT}/incubator/global-app-layer"
DIRECT_TARGET="$(direct_target)"

# Which examples to run.  Override with E2E_EXAMPLES="single-component frontend-postgres".
EXAMPLE_LIST="${E2E_EXAMPLES:-single-component frontend-postgres realistic-app gpu-eks-h100-training}"

PASS=0
FAIL=0
SKIP=0

section "Greenfield E2E: materialize config in ConfigHub, deliver to cluster"

step "Checking infrastructure"
require_infrastructure

for example in ${EXAMPLE_LIST}; do
  EXAMPLE_DIR="${EXAMPLES_DIR}/${example}"

  if [[ ! -d "${EXAMPLE_DIR}" ]]; then
    echo "  SKIP: ${example} — directory not found"
    SKIP=$((SKIP + 1))
    continue
  fi

  section "Greenfield: ${example}"

  step "${example}: setup (materialize config)"
  if ! (cd "${EXAMPLE_DIR}" && bash ./setup.sh "" "${DIRECT_TARGET}" 2>&1); then
    echo "  FAIL: ${example} setup failed" >&2
    FAIL=$((FAIL + 1))
    (cd "${EXAMPLE_DIR}" && bash ./cleanup.sh 2>/dev/null) || true
    continue
  fi

  step "${example}: verify (assert ConfigHub state)"
  if ! (cd "${EXAMPLE_DIR}" && bash ./verify.sh 2>&1); then
    echo "  FAIL: ${example} verify failed" >&2
    FAIL=$((FAIL + 1))
    (cd "${EXAMPLE_DIR}" && bash ./cleanup.sh 2>/dev/null) || true
    continue
  fi

  # Read the state to get the prefix.
  STATE_FILE="${EXAMPLE_DIR}/.state/state.env"
  if [[ ! -f "${STATE_FILE}" ]]; then
    echo "  FAIL: no state file after setup" >&2
    FAIL=$((FAIL + 1))
    continue
  fi

  # Extract PREFIX and TARGET_REF from state, and COMPONENTS + deploy_space from lib.
  # We run this in a subshell to avoid polluting our namespace.
  APPLY_INFO="$(bash -c '
    source "'"${STATE_FILE}"'"
    source "'"${EXAMPLE_DIR}/lib.sh"'"
    DEPLOY_NS="${DEPLOY_NAMESPACE:-cluster-a}"
    echo "DEPLOY_NS=${DEPLOY_NS}"
    echo "DEPLOY_SPACE=$(deploy_space)"
    for c in "${COMPONENTS[@]}"; do
      echo "UNIT:$(unit_name "${c}" deployment):${c}"
    done
  ' 2>/dev/null)" || {
    echo "  FAIL: could not extract apply info from state/lib" >&2
    FAIL=$((FAIL + 1))
    (cd "${EXAMPLE_DIR}" && bash ./cleanup.sh 2>/dev/null) || true
    continue
  }

  DEPLOY_NS="$(echo "${APPLY_INFO}" | grep '^DEPLOY_NS=' | cut -d= -f2)"
  DEPLOY_SPACE="$(echo "${APPLY_INFO}" | grep '^DEPLOY_SPACE=' | cut -d= -f2)"

  ensure_namespace "${DEPLOY_NS}"

  step "${example}: apply deployment units to cluster"
  APPLY_OK=true
  while IFS=: read -r tag unit_slug component; do
    [[ "${tag}" == "UNIT" ]] || continue
    echo "  Applying ${DEPLOY_SPACE}/${unit_slug}"
    if ! cub unit apply --space "${DEPLOY_SPACE}" "${unit_slug}" 2>&1; then
      echo "  FAIL: apply failed for ${unit_slug}" >&2
      APPLY_OK=false
    fi
  done <<< "${APPLY_INFO}"

  if [[ "${APPLY_OK}" != "true" ]]; then
    FAIL=$((FAIL + 1))
    (cd "${EXAMPLE_DIR}" && bash ./cleanup.sh 2>/dev/null) || true
    continue
  fi

  step "${example}: waiting for apply to propagate"
  sleep 15

  step "${example}: verifying resources on cluster"
  CLUSTER_OK=true
  while IFS=: read -r tag unit_slug component; do
    [[ "${tag}" == "UNIT" ]] || continue
    if kubectl get deployment "${component}" -n "${DEPLOY_NS}" >/dev/null 2>&1; then
      echo "  OK: deployment/${component} exists in ${DEPLOY_NS}"
    elif kubectl get statefulset "${component}" -n "${DEPLOY_NS}" >/dev/null 2>&1; then
      echo "  OK: statefulset/${component} exists in ${DEPLOY_NS}"
    elif kubectl get daemonset "${component}" -n "${DEPLOY_NS}" >/dev/null 2>&1; then
      echo "  OK: daemonset/${component} exists in ${DEPLOY_NS}"
    else
      echo "  WARN: no deployment/statefulset/daemonset named '${component}' in ${DEPLOY_NS}"
    fi
  done <<< "${APPLY_INFO}"

  step "${example}: cleanup (ConfigHub spaces)"
  (cd "${EXAMPLE_DIR}" && bash ./cleanup.sh 2>&1) || true

  if [[ "${CLUSTER_OK}" == "true" ]]; then
    PASS=$((PASS + 1))
    echo "  ${example}: PASSED"
  else
    FAIL=$((FAIL + 1))
    echo "  ${example}: FAILED"
  fi

  # Clean up namespace between examples to avoid resource collisions.
  kubectl delete namespace "${DEPLOY_NS}" --wait=false 2>/dev/null || true
  sleep 5
  ensure_namespace "${DEPLOY_NS}"
done

# Final namespace cleanup.
step "Cleaning up cluster namespace"
kubectl delete namespace "${DEPLOY_NS:-cluster-a}" --wait=false 2>/dev/null || true

section "Greenfield E2E: ${PASS} passed, ${FAIL} failed, ${SKIP} skipped"

if [[ "${FAIL}" -gt 0 ]]; then
  exit 1
fi
