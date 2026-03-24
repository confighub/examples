#!/usr/bin/env bash
# Remove all ConfigHub objects created by this example.
#
# Deletes all spaces labeled ExampleName=platform-write-api.

set -euo pipefail

CUB="${CUB:-cub}"
EXAMPLE_LABEL="platform-write-api"

command -v "${CUB}" >/dev/null 2>&1 || { echo "error: cub CLI not found." >&2; exit 1; }

echo "Cleaning up platform-write-api example spaces..."

spaces=$(${CUB} space list --where "Labels.ExampleName = '${EXAMPLE_LABEL}'" --json 2>/dev/null | jq -r '.[].Space.Slug' 2>/dev/null || true)

if [[ -z "$spaces" ]]; then
  echo "  No spaces found with ExampleName=${EXAMPLE_LABEL}. Nothing to clean up."
  exit 0
fi

for space in $spaces; do
  # Delete units first (spaces with mutations can't be deleted directly)
  units=$(${CUB} unit list --space "${space}" --json 2>/dev/null | jq -r '.[].Unit.Slug' 2>/dev/null || true)
  for unit in $units; do
    ${CUB} unit delete --space "${space}" "${unit}" --quiet 2>/dev/null || true
  done
  # Delete targets (after units, since units reference targets)
  targets=$(${CUB} target list --space "${space}" --json 2>/dev/null | jq -r '.[].Target.Slug' 2>/dev/null || true)
  for target in $targets; do
    ${CUB} target delete --space "${space}" "${target}" --quiet 2>/dev/null || true
  done
  # Delete workers if present
  workers=$(${CUB} worker list --space "${space}" --json 2>/dev/null | jq -r '.[].BridgeWorker.Slug' 2>/dev/null || true)
  for worker in $workers; do
    ${CUB} worker delete --space "${space}" "${worker}" --quiet 2>/dev/null || true
  done
  ${CUB} space delete "${space}" --quiet 2>/dev/null || true
  echo "  Deleted: ${space}"
done

echo "Done."
