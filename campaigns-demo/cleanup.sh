#!/usr/bin/env bash
# cleanup.sh — Remove all campaigns-demo data from ConfigHub
#
# Deletes the space (and everything in it) created by setup.sh.
#
# Usage:
#   ./cleanup.sh
#   SPACE=my-space ./cleanup.sh

set -euo pipefail

export CONFIGHUB_URL="${CONFIGHUB_URL:-https://app.confighub.com}"
SPACE="${SPACE:-campaigns-demo}"
cub="${CUB:-cub}"

if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found." >&2
  exit 1
fi

if ! $cub auth get-token &>/dev/null; then
  echo "ERROR: Not authenticated. Run: $cub auth login" >&2
  exit 1
fi

echo "Deleting space '$SPACE' and all its contents..."
if $cub space delete "$SPACE" --force 2>/dev/null; then
  echo "Done."
else
  echo "Space '$SPACE' not found (already deleted?)."
fi
