#!/usr/bin/env bash
# Cleanup for springboot-platform-platform-centric
#
# Deletes all ConfigHub objects created by setup.sh

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-platform-centric"

echo "=== Cleaning up springboot-platform (platform-centric) ==="
echo ""

# Get all spaces with our label
spaces=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json 2>/dev/null | jq -r '.[].Space.Slug' || echo "")

if [[ -z "${spaces}" ]]; then
  echo "No spaces found with label ExampleName=${EXAMPLE_LABEL}"
  exit 0
fi

echo "Deleting spaces:"
for space in ${spaces}; do
  echo "  Deleting: ${space}"
  echo "y" | ${CUB} space delete "${space}" --quiet 2>/dev/null || true
done

echo ""
echo "=== Cleanup complete ==="
