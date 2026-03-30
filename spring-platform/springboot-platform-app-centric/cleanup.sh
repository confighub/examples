#!/usr/bin/env bash
# Cleanup springboot-platform-app-centric
#
# Delegates to the parent example's cleanup script.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="${SCRIPT_DIR}/../springboot-platform-app"

if [[ ! -f "${PARENT_DIR}/confighub-cleanup.sh" ]]; then
  echo "error: Parent cleanup script not found at ${PARENT_DIR}/confighub-cleanup.sh" >&2
  exit 1
fi

exec "${PARENT_DIR}/confighub-cleanup.sh"
