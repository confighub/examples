#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CONTROL_SPACE="${GITOPS_ARGO_EXPERT_CONTROL_SPACE:-gitops-argo-expert-control}"
DEV_SPACE="${GITOPS_ARGO_EXPERT_DEV_SPACE:-gitops-argo-expert-dev}"
STAGING_SPACE="${GITOPS_ARGO_EXPERT_STAGING_SPACE:-gitops-argo-expert-staging}"
PROD_SPACE="${GITOPS_ARGO_EXPERT_PROD_SPACE:-gitops-argo-expert-prod}"

echo "This removes local rendered output only. It does not delete anything in ConfigHub."
echo "To remove the ConfigHub Spaces this example created, run:"
echo "  cub space delete $CONTROL_SPACE"
echo "  cub space delete $DEV_SPACE"
echo "  cub space delete $STAGING_SPACE"
echo "  cub space delete $PROD_SPACE"

rm -rf "$VAR_DIR"

echo "Removed local rendered output under var/."
