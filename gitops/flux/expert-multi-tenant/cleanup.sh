#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
PLATFORM_SPACE="${GITOPS_FLUX_MULTI_TENANT_PLATFORM_SPACE:-gitops-flux-multi-tenant-platform}"
STOREFRONT_BOOTSTRAP_SPACE="${GITOPS_FLUX_MULTI_TENANT_STOREFRONT_BOOTSTRAP_SPACE:-gitops-flux-multi-tenant-team-storefront-bootstrap}"
STOREFRONT_WORKLOADS_SPACE="${GITOPS_FLUX_MULTI_TENANT_STOREFRONT_WORKLOADS_SPACE:-gitops-flux-multi-tenant-team-storefront-workloads}"
PAYMENTS_BOOTSTRAP_SPACE="${GITOPS_FLUX_MULTI_TENANT_PAYMENTS_BOOTSTRAP_SPACE:-gitops-flux-multi-tenant-team-payments-bootstrap}"
PAYMENTS_WORKLOADS_SPACE="${GITOPS_FLUX_MULTI_TENANT_PAYMENTS_WORKLOADS_SPACE:-gitops-flux-multi-tenant-team-payments-workloads}"
LOYALTY_BOOTSTRAP_SPACE="${GITOPS_FLUX_MULTI_TENANT_LOYALTY_BOOTSTRAP_SPACE:-gitops-flux-multi-tenant-team-loyalty-bootstrap}"
LOYALTY_WORKLOADS_SPACE="${GITOPS_FLUX_MULTI_TENANT_LOYALTY_WORKLOADS_SPACE:-gitops-flux-multi-tenant-team-loyalty-workloads}"

echo "This removes local rendered output only. It does not delete anything in ConfigHub."
echo "To remove the ConfigHub Spaces this example created, run:"
echo "  cub space delete --recursive --detach $PLATFORM_SPACE"
echo "  cub space delete --recursive --detach $STOREFRONT_BOOTSTRAP_SPACE"
echo "  cub space delete --recursive --detach $STOREFRONT_WORKLOADS_SPACE"
echo "  cub space delete --recursive --detach $PAYMENTS_BOOTSTRAP_SPACE"
echo "  cub space delete --recursive --detach $PAYMENTS_WORKLOADS_SPACE"
echo "  cub space delete --recursive --detach $LOYALTY_BOOTSTRAP_SPACE"
echo "  cub space delete --recursive --detach $LOYALTY_WORKLOADS_SPACE"

rm -rf "$VAR_DIR"

echo "Removed local rendered output under var/."
