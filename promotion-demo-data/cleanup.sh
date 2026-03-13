#!/bin/bash
# ConfigHub Demo Cleanup
#
# Deletes all demo data by label. This removes all spaces (and their units,
# targets, workers, etc.) that were created by setup.sh.
#
# Usage:
#   ./cleanup.sh
#   CUB=/path/to/cub ./cleanup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"

echo "=== ConfigHub Demo Cleanup ==="
echo ""
echo "This will delete ALL spaces with label ExampleName=${EXAMPLE_NAME}"
echo ""

$CUB space delete --where "Labels.ExampleName = '${EXAMPLE_NAME}'" --recursive

echo ""
echo "=== Cleanup complete ==="
