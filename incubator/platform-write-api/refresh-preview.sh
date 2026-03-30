#!/usr/bin/env bash
# Preview what happens when the generator re-renders.
#
# Thin wrapper around springboot-platform-app refresh-preview script.
#
# Usage:
#   ./refresh-preview.sh          # Prod (default)
#   ./refresh-preview.sh dev      # Dev
#   ./refresh-preview.sh --json   # Machine-readable

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPRING_DIR="${SCRIPT_DIR}/../spring-platform/springboot-platform-app"

if [[ ! -x "${SPRING_DIR}/confighub-refresh-preview.sh" ]]; then
  echo "error: ../spring-platform/springboot-platform-app/confighub-refresh-preview.sh not found" >&2
  exit 1
fi

exec "${SPRING_DIR}/confighub-refresh-preview.sh" "$@"
