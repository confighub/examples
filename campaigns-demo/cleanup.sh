#!/usr/bin/env bash
# cleanup.sh — Tear down the campaigns-demo space in a single pass.
#
# Handles the dependency ordering needed to fully delete the space:
#   1. Find bridge workers that live in this space.
#   2. Delete any Trigger — in ANY space — that references those workers.
#      (ConfigHub blocks worker deletes while a Trigger references them,
#       and earlier runs of the Campaigns UI can leave orphans in other
#       spaces pointing at this space's worker.)
#   3. `cub space delete --recursive-force` to nuke units, views, filters,
#      workers, etc. in one shot.
#
# Usage:
#   ./cleanup.sh
#   CONFIGHUB_URL=http://localhost:9090 SPACE=campaigns-demo ./cleanup.sh

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

API_TOKEN=$($cub auth get-token)

api() {
  local method="$1" path="$2"
  shift 2
  curl -sf -X "$method" \
    -H "Authorization: Bearer ${API_TOKEN}" \
    -H "Content-Type: application/json" \
    "$@" "${CONFIGHUB_URL}/api${path}"
}

# ── Resolve space ─────────────────────────────────────────────────────────────

SPACE_ID=$($cub space get "$SPACE" --jq ".Space.SpaceID" --quiet 2>/dev/null || true)
if [[ -z "$SPACE_ID" ]]; then
  echo "Space '$SPACE' not found — nothing to clean up."
  exit 0
fi

echo "Cleaning up space '$SPACE' ($SPACE_ID)..."

# ── Clear cross-space trigger references on our workers ─────────────────────
#
# Workers in this space may be referenced by Triggers in OTHER spaces (e.g.
# leftovers from earlier Campaigns-UI test runs). Those references block the
# recursive space delete. Walk each worker, find referencing triggers, and
# delete them so the worker (and therefore the space) can be removed.

worker_ids=$(api GET "/space/${SPACE_ID}/bridge_worker" 2>/dev/null \
  | jq -r '.[].BridgeWorker.BridgeWorkerID // empty')

if [[ -n "$worker_ids" ]]; then
  total_orphans=0
  while IFS= read -r wid; do
    [[ -z "$wid" ]] && continue
    # URL-encode "BridgeWorkerID = '<uuid>'"
    where_encoded="BridgeWorkerID+%3D+%27${wid}%27"
    orphans=$(api GET "/trigger?where=${where_encoded}" 2>/dev/null \
      | jq -r '.[] | "\(.Trigger.SpaceID)|\(.Trigger.TriggerID)"' 2>/dev/null || true)
    [[ -z "$orphans" ]] && continue
    while IFS='|' read -r sid tid; do
      [[ -z "$sid" || -z "$tid" ]] && continue
      if api DELETE "/space/${sid}/trigger/${tid}" >/dev/null 2>&1; then
        total_orphans=$((total_orphans + 1))
      fi
    done <<< "$orphans"
  done <<< "$worker_ids"
  if (( total_orphans > 0 )); then
    echo "  - Cleared $total_orphans cross-space trigger reference(s) on this space's workers."
  fi
fi

# ── Recursively delete the space ─────────────────────────────────────────────

if $cub space delete "$SPACE" --recursive-force; then
  echo "Done."
else
  echo "ERROR: Could not delete space '$SPACE'. Investigate remaining references." >&2
  exit 1
fi
