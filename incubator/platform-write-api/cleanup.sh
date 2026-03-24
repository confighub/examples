#!/usr/bin/env bash
# Remove all ConfigHub objects created by this example.
#
# Deletes all spaces labeled ExampleName=platform-write-api.

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="platform-write-api"

command -v "${CUB}" >/dev/null 2>&1 || { echo "error: cub CLI not found." >&2; exit 1; }

echo "Cleaning up platform-write-api example spaces..."

spaces=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json 2>/dev/null | jq -r '.[].Slug' 2>/dev/null || true)

if [[ -z "$spaces" ]]; then
  echo "  No spaces found with ExampleName=${EXAMPLE_LABEL}. Nothing to clean up."
  exit 0
fi

for space in $spaces; do
  ${CUB} space delete "${space}" --quiet 2>/dev/null || true
  echo "  Deleted: ${space}"
done

echo "Done."
