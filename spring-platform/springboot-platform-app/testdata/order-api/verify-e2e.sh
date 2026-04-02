#!/usr/bin/env bash
# End-to-end verification for springboot-platform-app
#
# Verifies that the application is actually deployed and responding.
# This is REAL verification - it hits the actual running pod.
#
# Usage:
#   ./verify-e2e.sh              # Full verification with HTTP check
#   ./verify-e2e.sh --quick      # Skip HTTP check, just verify pod status
#   ./verify-e2e.sh --json       # Output results as JSON
#
# Prerequisites:
#   - Kind cluster running (./bin/create-cluster)
#   - Image built and loaded (./bin/build-image)
#   - ConfigHub setup complete (./confighub-setup.sh --with-targets)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="springboot-platform"
DEPLOY_NAMESPACE="order-api"
SERVICE_NAME="order-api"
ENDPOINT="/api/inventory/summary"
LOCAL_PORT=18080
CUB="${CUB:-cub}"
DEFAULT_KUBECONFIG_PATH="${SCRIPT_DIR}/var/${CLUSTER_NAME}.kubeconfig"
WORKER_PID_FILE="${SCRIPT_DIR}/var/worker.pid"

QUICK=false
JSON_OUTPUT=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --quick) QUICK=true; shift ;;
    --json) JSON_OUTPUT=true; shift ;;
    *) shift ;;
  esac
done

# Track results using simple variables (bash 3.2 compatible)
RESULT_cluster="unknown"
RESULT_namespace="unknown"
RESULT_deployment="unknown"
RESULT_pods="unknown"
RESULT_confighub_unit="unknown"
RESULT_http_response="unknown"
RESULT_reservation_mode="unknown"

errors=0

log() {
  if [[ "${JSON_OUTPUT}" == "false" ]]; then
    echo "$@"
  fi
}

check_pass() {
  log "  OK: $1"
}

check_fail() {
  log "  FAIL: $1" >&2
  errors=$((errors + 1))
}

if [[ -z "${KUBECONFIG:-}" && -f "${DEFAULT_KUBECONFIG_PATH}" ]]; then
  export KUBECONFIG="${DEFAULT_KUBECONFIG_PATH}"
  log "Using kubeconfig: ${KUBECONFIG}"
fi

# Check 1: Cluster reachable
log "Checking cluster..."
if kubectl cluster-info >/dev/null 2>&1; then
  RESULT_cluster="reachable"
  check_pass "Cluster is reachable"
else
  RESULT_cluster="unreachable"
  check_fail "Cluster not reachable. Run ./bin/create-cluster and export KUBECONFIG."
fi

# Check 2: Namespace exists
log "Checking namespace..."
if kubectl get namespace "${DEPLOY_NAMESPACE}" >/dev/null 2>&1; then
  RESULT_namespace="exists"
  check_pass "Namespace '${DEPLOY_NAMESPACE}' exists"
else
  RESULT_namespace="missing"
  check_fail "Namespace '${DEPLOY_NAMESPACE}' not found"
fi

# Check 3: Deployment exists and is available
log "Checking deployment..."
DEPLOYMENT_STATUS=$(kubectl get deployment "${SERVICE_NAME}" -n "${DEPLOY_NAMESPACE}" -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null || echo "NotFound")
if [[ "${DEPLOYMENT_STATUS}" == "True" ]]; then
  RESULT_deployment="available"
  check_pass "Deployment '${SERVICE_NAME}' is Available"
elif [[ "${DEPLOYMENT_STATUS}" == "NotFound" ]]; then
  RESULT_deployment="missing"
  check_fail "Deployment '${SERVICE_NAME}' not found"
else
  RESULT_deployment="not-ready"
  check_fail "Deployment '${SERVICE_NAME}' is not Available (status: ${DEPLOYMENT_STATUS})"
fi

# Check 4: Pods are running
log "Checking pods..."
RUNNING_PODS=$(kubectl get pods -n "${DEPLOY_NAMESPACE}" -l app.kubernetes.io/name=order-api --field-selector=status.phase=Running -o name 2>/dev/null | wc -l | tr -d ' ')
if [[ "${RUNNING_PODS}" -gt 0 ]]; then
  RESULT_pods="${RUNNING_PODS} running"
  check_pass "${RUNNING_PODS} pod(s) running"
else
  RESULT_pods="0 running"
  check_fail "No running pods found"
  log "    Pod status:"
  kubectl get pods -n "${DEPLOY_NAMESPACE}" -l app.kubernetes.io/name=order-api 2>/dev/null | sed 's/^/    /' || true
fi

# Check 5: ConfigHub unit exists
log "Checking ConfigHub unit..."
if UNIT_JSON=$(${CUB} unit get --space order-api-prod --json order-api 2>/dev/null); then
  RESULT_confighub_unit="exists"
  check_pass "ConfigHub unit 'order-api-prod/order-api' exists"

  # Check for target in multiple possible locations
  TARGET_SLUG=$(echo "${UNIT_JSON}" | jq -r '.Target.Slug // .UnitStatus.TargetRef // ""')
  TARGET_PROVIDER=$(echo "${UNIT_JSON}" | jq -r '.Target.ProviderType // ""')

  if [[ -n "${TARGET_SLUG}" ]]; then
    check_pass "Prod unit target = ${TARGET_SLUG}"

    # Use tr for lowercase (bash 3.2 compatible)
    TARGET_PROVIDER_LOWER=$(echo "${TARGET_PROVIDER}" | tr '[:upper:]' '[:lower:]')
    if [[ "${TARGET_PROVIDER_LOWER}" == "kubernetes" ]]; then
      check_pass "Bound target provider is Kubernetes"
    else
      check_fail "Bound target provider is '${TARGET_PROVIDER}', expected Kubernetes"
    fi
  else
    check_fail "Prod unit is not bound to any target"
  fi
else
  RESULT_confighub_unit="missing"
  check_fail "ConfigHub unit not found"
fi

log "Checking local worker process..."
if [[ -f "${WORKER_PID_FILE}" ]] && kill -0 "$(cat "${WORKER_PID_FILE}")" 2>/dev/null; then
  check_pass "Local worker process is running (PID $(cat "${WORKER_PID_FILE}"))"
else
  log "  NOTE: local worker PID not found or not running; relying on target and cluster evidence"
fi

# Check 6: HTTP response (unless --quick)
if [[ "${QUICK}" == "false" && "${RESULT_pods}" != "0 running" ]]; then
  log "Checking HTTP response..."

  # Start port-forward in background
  kubectl port-forward -n "${DEPLOY_NAMESPACE}" "svc/${SERVICE_NAME}" "${LOCAL_PORT}:80" >/dev/null 2>&1 &
  PF_PID=$!

  # Wait for port-forward to be ready
  sleep 2

  # Make HTTP request
  HTTP_RESPONSE=$(curl -sf "http://localhost:${LOCAL_PORT}${ENDPOINT}" 2>/dev/null || echo "")

  # Kill port-forward
  kill "${PF_PID}" 2>/dev/null || true
  wait "${PF_PID}" 2>/dev/null || true

  if [[ -n "${HTTP_RESPONSE}" ]]; then
    RESULT_http_response="success"
    check_pass "HTTP response received from ${ENDPOINT}"

    # Extract reservationMode
    RESERVATION_MODE=$(echo "${HTTP_RESPONSE}" | jq -r '.reservationMode // empty' 2>/dev/null || echo "")
    if [[ -n "${RESERVATION_MODE}" ]]; then
      RESULT_reservation_mode="${RESERVATION_MODE}"
      check_pass "reservationMode = ${RESERVATION_MODE}"
    else
      RESULT_reservation_mode="not-found"
      check_fail "Could not extract reservationMode from response"
    fi

    # Show full response
    if [[ "${JSON_OUTPUT}" == "false" ]]; then
      log ""
      log "Full HTTP response:"
      echo "${HTTP_RESPONSE}" | jq . 2>/dev/null || echo "${HTTP_RESPONSE}"
    fi
  else
    RESULT_http_response="failed"
    RESULT_reservation_mode="unknown"
    check_fail "No HTTP response from ${ENDPOINT}"
  fi
elif [[ "${QUICK}" == "true" ]]; then
  log "Skipping HTTP check (--quick mode)"
  RESULT_http_response="skipped"
  RESULT_reservation_mode="skipped"
fi

# Output JSON if requested
if [[ "${JSON_OUTPUT}" == "true" ]]; then
  cat <<ENDJSON
{
  "cluster": "${RESULT_cluster}",
  "namespace": "${RESULT_namespace}",
  "deployment": "${RESULT_deployment}",
  "pods": "${RESULT_pods}",
  "confighub_unit": "${RESULT_confighub_unit}",
  "http_response": "${RESULT_http_response}",
  "reservation_mode": "${RESULT_reservation_mode}",
  "errors": ${errors}
}
ENDJSON
  exit ${errors}
fi

# Summary
log ""
if [[ ${errors} -eq 0 ]]; then
  log "========================================="
  log "All checks passed!"
  log "========================================="
  log ""
  log "The application is deployed and responding."
  log "reservationMode = ${RESULT_reservation_mode}"
  log ""
  log "To test a mutation:"
  log "  cub function do --space order-api-prod \\"
  log "    --change-desc 'test: change reservation mode' \\"
  log "    -- set-env order-api 'FEATURE_INVENTORY_RESERVATIONMODE=optimistic'"
  log "  cub unit apply --space order-api-prod order-api"
  log "  kubectl rollout status deployment/order-api -n ${DEPLOY_NAMESPACE}"
  log "  ./verify-e2e.sh"
  log "========================================="
else
  log "========================================="
  log "Verification failed with ${errors} error(s)"
  log "========================================="
  log ""
  log "Troubleshooting:"
  log "  kubectl get pods -n ${DEPLOY_NAMESPACE}"
  log "  kubectl describe deployment ${SERVICE_NAME} -n ${DEPLOY_NAMESPACE}"
  log "  kubectl logs -n ${DEPLOY_NAMESPACE} -l app.kubernetes.io/name=order-api"
  log "========================================="
fi

exit ${errors}
