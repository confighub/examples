#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
DEV_SPACE="${GITOPS_ARGO_BEGINNER_AOA_DEV_SPACE:-gitops-argo-beginner-aoa-dev}"
PROD_SPACE="${GITOPS_ARGO_BEGINNER_AOA_PROD_SPACE:-gitops-argo-beginner-aoa-prod}"

echo "This removes local rendered output only. It does not delete anything in ConfigHub."
echo "To remove the ConfigHub Spaces this example created, run:"
echo "  cub space delete $DEV_SPACE"
echo "  cub space delete $PROD_SPACE"

rm -rf "$VAR_DIR"

echo "Removed local rendered output under var/."
