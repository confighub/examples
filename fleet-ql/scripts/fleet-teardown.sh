#!/usr/bin/env bash
# Remove the acme-* fleet seeded by fleet-setup.sh (ConfigHub only).
set -euo pipefail
cub=cub
PREFIX=acme

for comp in storefront orders payments; do
  for env in dev staging prod; do
    space="${PREFIX}-${comp}-${env}"
    echo "deleting ${space}"
    $cub space delete "$space" --recursive >/dev/null 2>&1 || true
  done
done
echo "Done."
