#!/usr/bin/env bash
# ConfigHub cleanup for springboot-platform-app
#
# Deletes all spaces created by confighub-setup.sh using the ExampleName label.
#
# Usage:
#   ./confighub-cleanup.sh

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app"

echo "=== ConfigHub cleanup for springboot-platform-app ==="
echo ""
echo "This will delete ALL spaces with label ExampleName=${EXAMPLE_LABEL}"
echo ""

${CUB} space delete --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --recursive

echo ""
echo "=== Cleanup complete ==="
