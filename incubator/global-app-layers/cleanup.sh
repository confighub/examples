#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_cub

if ! state_exists; then
  echo "No state file found. Nothing to clean up." >&2
  exit 1
fi

load_state

echo "==> Deleting spaces for ExampleChain=${PREFIX}"
cub space delete --where "Labels.ExampleChain = '${PREFIX}'" --recursive
rm -rf "${STATE_DIR}"

echo "Cleaned up global-app-layers chain for prefix ${PREFIX}."
