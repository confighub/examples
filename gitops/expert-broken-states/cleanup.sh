#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
ARGO_CONTROL_SPACE="${GITOPS_EXPERT_BROKEN_STATES_ARGO_CONTROL_SPACE:-gitops-expert-broken-states-argo-control}"
FLUX_CONTROL_SPACE="${GITOPS_EXPERT_BROKEN_STATES_FLUX_CONTROL_SPACE:-gitops-expert-broken-states-flux-control}"
WORKLOAD_SPACE="${GITOPS_EXPERT_BROKEN_STATES_SPACE:-gitops-expert-broken-states}"

echo "This removes local rendered output only. It does not delete anything in ConfigHub."
echo "To remove the ConfigHub Spaces this example created, run:"
echo "  cub space delete $ARGO_CONTROL_SPACE"
echo "  cub space delete $FLUX_CONTROL_SPACE"
echo "  cub space delete $WORKLOAD_SPACE"

rm -rf "$VAR_DIR"

echo "Removed local rendered output under var/."
