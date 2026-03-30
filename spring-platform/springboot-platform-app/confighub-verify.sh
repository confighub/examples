#!/usr/bin/env bash
# Verify that ConfigHub objects exist and are inspectable.
#
# Usage:
#   ./confighub-verify.sh                   # Verify spaces and units only
#   ./confighub-verify.sh --targets         # Also verify real Kubernetes deployment
#   ./confighub-verify.sh --noop-targets    # Also verify Noop targets and infra space

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app"
DEPLOY_NAMESPACE="inventory-api"
DEFAULT_KUBECONFIG_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/var/springboot-platform.kubeconfig"
ENVS=(dev stage prod)
MODE="confighub-only"
errors=0

case "${1:-}" in
  --targets) MODE="real-targets" ;;
  --noop-targets) MODE="noop-targets" ;;
esac

echo "=== Verifying ConfigHub objects for springboot-platform-app ==="
echo "Mode: ${MODE}"
echo ""

# Check spaces exist
space_count=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json | jq 'length')
case "${MODE}" in
  "noop-targets") expected=4 ;;  # 3 env + 1 infra
  *) expected=3 ;;               # 3 env spaces
esac

if [[ "${space_count}" -lt "${expected}" ]]; then
  echo "FAIL: expected at least ${expected} spaces, found ${space_count}" >&2
  errors=$((errors + 1))
else
  echo "ok: found ${space_count} spaces with ExampleName=${EXAMPLE_LABEL}"
fi

# Check each env space has the inventory-api unit
for env in "${ENVS[@]}"; do
  space="inventory-api-${env}"
  unit_json=$(${CUB} unit get --space "${space}" --json inventory-api 2>&1) || {
    echo "FAIL: unit inventory-api not found in space ${space}" >&2
    errors=$((errors + 1))
    continue
  }

  # Check unit has data content
  data_len=$(echo "${unit_json}" | jq '.Unit.Data | length')
  if [[ "${data_len}" -lt 1 ]]; then
    echo "FAIL: unit ${space}/inventory-api has no data" >&2
    errors=$((errors + 1))
  else
    echo "ok: ${space}/inventory-api has data (${data_len} bytes)"
  fi

  # For target modes, verify unit status
  if [[ "${MODE}" != "confighub-only" ]]; then
    status=$(echo "${unit_json}" | jq -r '.UnitStatus.Status // "Unknown"')
    sync=$(echo "${unit_json}" | jq -r '.UnitStatus.SyncStatus // "Unknown"')
    if [[ "${status}" != "Ready" && "${status}" != "Unknown" ]]; then
      echo "FAIL: ${space}/inventory-api status is ${status}, expected Ready" >&2
      errors=$((errors + 1))
    else
      echo "ok: ${space}/inventory-api status=${status} sync=${sync}"
    fi
  fi
done

# Mode-specific checks
case "${MODE}" in
  "noop-targets")
    echo ""
    echo "Verifying Noop target infrastructure..."

    # Check infra space exists
    infra_json=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json \
      | jq '[.[] | select(.Space.Slug == "inventory-api-infra")]')
    infra_count=$(echo "${infra_json}" | jq 'length')
    if [[ "${infra_count}" -lt 1 ]]; then
      echo "FAIL: infra space inventory-api-infra not found" >&2
      errors=$((errors + 1))
    else
      echo "ok: infra space inventory-api-infra exists"
    fi

    # Check each env space has a Noop target
    for env in "${ENVS[@]}"; do
      space="inventory-api-${env}"
      target_count=$(${CUB} target list --space "${space}" --json 2>&1 | jq 'length')
      if [[ "${target_count}" -lt 1 ]]; then
        echo "FAIL: no target in ${space}" >&2
        errors=$((errors + 1))
      else
        echo "ok: ${space} has ${target_count} target(s)"
      fi
    done
    ;;

  "real-targets")
    echo ""
    echo "Verifying real Kubernetes deployment..."

    if [[ -z "${KUBECONFIG:-}" && -f "${DEFAULT_KUBECONFIG_PATH}" ]]; then
      export KUBECONFIG="${DEFAULT_KUBECONFIG_PATH}"
      echo "Using kubeconfig: ${KUBECONFIG}"
    fi

    # Check kubectl is available and cluster reachable
    if ! command -v kubectl >/dev/null 2>&1; then
      echo "FAIL: kubectl not found" >&2
      errors=$((errors + 1))
    elif ! kubectl cluster-info >/dev/null 2>&1; then
      echo "FAIL: Kubernetes cluster not reachable" >&2
      errors=$((errors + 1))
    else
      echo "ok: Kubernetes cluster is reachable"

      # Check namespace exists
      if kubectl get namespace "${DEPLOY_NAMESPACE}" >/dev/null 2>&1; then
        echo "ok: namespace '${DEPLOY_NAMESPACE}' exists"
      else
        echo "FAIL: namespace '${DEPLOY_NAMESPACE}' not found" >&2
        errors=$((errors + 1))
      fi

      # Check deployment exists and is available
      DEPLOYMENT_STATUS=$(kubectl get deployment inventory-api -n "${DEPLOY_NAMESPACE}" \
        -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null || echo "NotFound")
      if [[ "${DEPLOYMENT_STATUS}" == "True" ]]; then
        echo "ok: deployment 'inventory-api' is Available"
      elif [[ "${DEPLOYMENT_STATUS}" == "NotFound" ]]; then
        echo "FAIL: deployment 'inventory-api' not found" >&2
        errors=$((errors + 1))
      else
        echo "FAIL: deployment 'inventory-api' is not Available (status: ${DEPLOYMENT_STATUS})" >&2
        errors=$((errors + 1))
      fi

      # Check pods are running
      RUNNING_PODS=$(kubectl get pods -n "${DEPLOY_NAMESPACE}" -l app=inventory-api \
        --field-selector=status.phase=Running -o name 2>/dev/null | wc -l | tr -d ' ')
      if [[ "${RUNNING_PODS}" -gt 0 ]]; then
        echo "ok: ${RUNNING_PODS} pod(s) running"
      else
        echo "FAIL: no running pods found" >&2
        kubectl get pods -n "${DEPLOY_NAMESPACE}" -l app=inventory-api 2>/dev/null | sed 's/^/    /' || true
        errors=$((errors + 1))
      fi

      # Check prod unit is bound to a real target
      PROD_TARGET=$(${CUB} unit get --space inventory-api-prod --json inventory-api 2>/dev/null | \
        jq -r '.UnitStatus.TargetRef // "none"')
      if [[ "${PROD_TARGET}" != "none" && "${PROD_TARGET}" != "null" && -n "${PROD_TARGET}" ]]; then
        echo "ok: inventory-api-prod/inventory-api bound to target: ${PROD_TARGET}"

        TARGET_SPACE="${PROD_TARGET%/*}"
        TARGET_SLUG="${PROD_TARGET#*/}"
        TARGET_PROVIDER=$(${CUB} target list --space "${TARGET_SPACE}" --json 2>/dev/null | \
          jq -r --arg slug "${TARGET_SLUG}" '[.[] | select(.Target.Slug == $slug)][0].Target.ProviderType // "unknown"')
        if [[ "${TARGET_PROVIDER,,}" == "kubernetes" ]]; then
          echo "ok: bound target provider is Kubernetes"
        else
          echo "FAIL: bound target provider is '${TARGET_PROVIDER}', expected Kubernetes" >&2
          errors=$((errors + 1))
        fi
      else
        echo "FAIL: inventory-api-prod/inventory-api not bound to a target" >&2
        errors=$((errors + 1))
      fi
    fi
    ;;
esac

echo ""
if [[ "${errors}" -gt 0 ]]; then
  echo "FAIL: ${errors} error(s) found"
  exit 1
else
  case "${MODE}" in
    "real-targets")
      echo "ok: springboot-platform-app with real Kubernetes deployment is consistent"
      echo ""
      echo "Run ./verify-e2e.sh to test HTTP response from the deployed app."
      ;;
    "noop-targets")
      echo "ok: springboot-platform-app with Noop targets is consistent"
      ;;
    *)
      echo "ok: springboot-platform-app ConfigHub objects are consistent"
      ;;
  esac
fi
