#!/usr/bin/env bash
# Compare inventory-api config across dev, stage, prod.
#
# Thin wrapper around the springboot-platform-app compare script.
# Shows a side-by-side table with divergence markers.
#
# Usage:
#   ./compare.sh          # Table output
#   ./compare.sh --json   # Machine-readable

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPRING_DIR="${SCRIPT_DIR}/../spring-platform/springboot-platform-app"

if [[ ! -x "${SPRING_DIR}/confighub-compare.sh" ]]; then
  echo "error: ../spring-platform/springboot-platform-app/confighub-compare.sh not found" >&2
  echo "This example depends on the springboot-platform-app fixtures." >&2
  exit 1
fi

exec "${SPRING_DIR}/confighub-compare.sh" "$@"
