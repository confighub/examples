#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
PLATFORM_SPACE="${GITOPS_FLUX_MULTI_TENANT_PLATFORM_SPACE:-gitops-flux-multi-tenant-platform}"
STOREFRONT_SPACE="${GITOPS_FLUX_MULTI_TENANT_STOREFRONT_SPACE:-gitops-flux-multi-tenant-team-storefront}"
PAYMENTS_SPACE="${GITOPS_FLUX_MULTI_TENANT_PAYMENTS_SPACE:-gitops-flux-multi-tenant-team-payments}"
LOYALTY_SPACE="${GITOPS_FLUX_MULTI_TENANT_LOYALTY_SPACE:-gitops-flux-multi-tenant-team-loyalty}"

echo "This removes local rendered output only. It does not delete anything in ConfigHub."
echo "To remove the ConfigHub Spaces this example created, run:"
echo "  cub space delete $PLATFORM_SPACE"
echo "  cub space delete $STOREFRONT_SPACE"
echo "  cub space delete $PAYMENTS_SPACE"
echo "  cub space delete $LOYALTY_SPACE"

rm -rf "$VAR_DIR"

echo "Removed local rendered output under var/."
