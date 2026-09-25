#!/usr/bin/env bash
# Assert that a materialized global-app-layer example is present on the cluster.
# Usage: ./e2e/assert-cluster.sh <example-name>
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

example_name="${1:?Usage: assert-cluster.sh <example-name>}"
require_kubeconfig
require_example "${example_name}"
load_example "${example_name}"

ns="$(example_deploy_namespace)"

echo "==> Asserting cluster state for ${example_name}"
echo "    namespace: ${ns}"

found_any=false
for component in $(example_components); do
  if kubectl get deployment "${component}" -n "${ns}" >/dev/null 2>&1; then
    echo "  OK: deployment/${component} exists in ${ns}"
    found_any=true
    continue
  fi
  if kubectl get statefulset "${component}" -n "${ns}" >/dev/null 2>&1; then
    echo "  OK: statefulset/${component} exists in ${ns}"
    found_any=true
    continue
  fi
  if kubectl get daemonset "${component}" -n "${ns}" >/dev/null 2>&1; then
    echo "  OK: daemonset/${component} exists in ${ns}"
    found_any=true
    continue
  fi
  echo "  WARN: no deployment/statefulset/daemonset named '${component}' in ${ns}"
done

if [[ "${found_any}" != "true" ]]; then
  echo "FAIL: no expected workload resources were found for ${example_name} in ${ns}" >&2
  exit 1
fi

echo "==> Cluster assertions passed for ${example_name}"
