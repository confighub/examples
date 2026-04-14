#!/usr/bin/env bash
# cleanup.sh — Tear down the initiatives-demo space in a single pass.
#
# Handles the dependency ordering needed to fully delete the space:
#   1. Find bridge workers that live in this space.
#   2. Delete any Trigger — in ANY space — that references those workers.
#      (ConfigHub blocks worker deletes while a Trigger references them,
#       and earlier runs of the Initiatives UI can leave orphans in other
#       spaces pointing at this space's worker.)
#   3. `cub space delete --recursive-force` to nuke units, views, filters,
#      workers, etc. in one shot.
#
# Everything talks to ConfigHub via the `cub` CLI so the script automatically
# uses whichever server the current cub context points at.
#
# Usage:
#   ./cleanup.sh
#   SPACE=initiatives-demo ./cleanup.sh

set -euo pipefail

SPACE="${SPACE:-initiatives-demo}"
cub="${CUB:-cub}"

if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found." >&2
  exit 1
fi

if ! $cub auth get-token &>/dev/null; then
  echo "ERROR: Not authenticated. Run: $cub auth login" >&2
  exit 1
fi

# ── Resolve space ─────────────────────────────────────────────────────────────

if ! $cub space get "$SPACE" --quiet &>/dev/null; then
  echo "Space '$SPACE' not found — nothing to clean up."
  exit 0
fi

echo "Cleaning up space '$SPACE'..."

# ── Clear cross-space trigger references on our workers ─────────────────────
#
# Workers in this space may be referenced by Triggers in OTHER spaces (e.g.
# leftovers from earlier Initiatives-UI test runs). Those references block the
# recursive space delete. For each worker in this space, bulk-delete any
# trigger that references it, regardless of which space the trigger lives in.
#
# Best-effort: failures here must not block the main space delete.

worker_ids=$($cub worker list --space "$SPACE" \
  --jq '.[].BridgeWorker.BridgeWorkerID' --quiet 2>/dev/null || true)

if [[ -n "$worker_ids" ]]; then
  while IFS= read -r wid; do
    [[ -z "$wid" ]] && continue
    $cub trigger delete --space "*" \
      --where "BridgeWorkerID = '$wid'" --quiet >/dev/null 2>&1 || true
  done <<< "$worker_ids"
fi

# ── Recursively delete the space ─────────────────────────────────────────────

if $cub space delete "$SPACE" --recursive-force; then
  echo "Done."
else
  echo "ERROR: Could not delete space '$SPACE'. Investigate remaining references." >&2
  exit 1
fi
