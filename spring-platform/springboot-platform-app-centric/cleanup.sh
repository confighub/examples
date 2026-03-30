#!/usr/bin/env bash
# Cleanup for springboot-platform-app-centric
#
# Deletes all ConfigHub objects created by setup.sh

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app-centric"

echo "=== Cleaning up springboot-platform-app-centric ==="
echo ""
echo "This will delete ALL spaces with label ExampleName=${EXAMPLE_LABEL}"
echo ""

${CUB} space delete --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --recursive

echo ""
echo "=== Cleanup complete ==="
