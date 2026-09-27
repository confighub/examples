#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CONTROL_SPACE="${GITOPS_EXPERT_BROKEN_STATES_CONTROL_SPACE:-gitops-expert-broken-states-control}"
WORKLOAD_SPACE="${GITOPS_EXPERT_BROKEN_STATES_SPACE:-gitops-expert-broken-states}"

echo "This removes local rendered output only. It does not delete anything in ConfigHub."
echo "To remove the ConfigHub Spaces this example created, run:"
echo "  cub space delete $CONTROL_SPACE"
echo "  cub space delete $WORKLOAD_SPACE"

rm -rf "$VAR_DIR"

echo "Removed local rendered output under var/."
