#!/usr/bin/env bash
# Deliver an example's deployment units to the cluster via cub unit apply (direct mode).
#
# Usage: ./e2e/deliver-direct.sh <example-name>
#   e.g. ./e2e/deliver-direct.sh frontend-postgres
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

example_name="${1:?Usage: deliver-direct.sh <example-name>}"
require_kubeconfig
require_example "${example_name}"
load_example "${example_name}"

space="$(example_deploy_space)"
ns="$(example_deploy_namespace)"

echo "==> Delivering ${example_name} via apply:direct"
echo "    deploy space: ${space}"
echo "    namespace:    ${ns}"

ensure_namespace "${ns}"

for unit in $(example_deploy_units); do
  echo "==> Applying ${space}/${unit}"
  cub unit apply --space "${space}" "${unit}"
done

echo "==> Waiting for resources to appear (15s)..."
sleep 15

echo "==> Cluster state in namespace ${ns}:"
kubectl get all -n "${ns}" 2>/dev/null || true

echo ""
echo "deliver:direct complete for ${example_name}."
echo "Run ./e2e/assert-cluster.sh ${example_name} to verify."
