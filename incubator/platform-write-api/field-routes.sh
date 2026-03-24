#!/usr/bin/env bash
# Show field-level mutation routing for a given environment.
#
# Thin wrapper around springboot-platform-app field-routes script.
#
# Usage:
#   ./field-routes.sh          # Prod (default)
#   ./field-routes.sh dev      # Dev
#   ./field-routes.sh --json   # Machine-readable

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPRING_DIR="${SCRIPT_DIR}/../springboot-platform-app"

if [[ ! -x "${SPRING_DIR}/confighub-field-routes.sh" ]]; then
  echo "error: ../springboot-platform-app/confighub-field-routes.sh not found" >&2
  exit 1
fi

exec "${SPRING_DIR}/confighub-field-routes.sh" "$@"
