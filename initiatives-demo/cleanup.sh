#!/usr/bin/env bash
# cleanup.sh — Tear down the initiatives demo (component spaces + platform space)
#              and its kind cluster.
#
# The demo spans several spaces:
#   - component spaces (Labels.Purpose=<purpose>, Labels.Layer=App): aichat,
#     portal, eshop, docs, website — hold the application units.
#   - the platform space (default: initiatives-demo) — holds the workers,
#     filters, views, and triggers.
#
# Ordering matters:
#   1. Delete any Trigger — in ANY space — that references this demo's workers.
#      (ConfigHub blocks worker deletes while a Trigger references them.)
#   2. Delete the component spaces first (they reference the platform's trigger
#      filter via TriggerFilterID; clearing them before the platform avoids
#      dangling references).
#   3. Delete the platform space (units/views/filters/triggers/workers) with
#      --recursive-force.
#   4. Delete the local kind cluster created by setup.sh (best effort).
#
# Everything talks to ConfigHub via the `cub` CLI so the script automatically
# uses whichever server the current cub context points at.
#
# Usage:
#   ./cleanup.sh
#   PLATFORM_SPACE=initiatives-demo PURPOSE=initiatives-demo ./cleanup.sh

set -euo pipefail

PLATFORM_SPACE="${PLATFORM_SPACE:-${SPACE:-initiatives-demo}}"
PURPOSE="${PURPOSE:-initiatives-demo}"
CLUSTER_NAME="${CLUSTER_NAME:-$PLATFORM_SPACE}"
cub="${CUB:-cub}"

if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found." >&2
  exit 1
fi

if ! $cub auth get-token &>/dev/null; then
  echo "ERROR: Not authenticated. Run: $cub auth login" >&2
  exit 1
fi

# ── Clear cross-space trigger references on our workers ─────────────────────
#
# Workers in the platform space may be referenced by Triggers in OTHER spaces.
# Those references block the recursive space delete. For each worker in the
# platform space, bulk-delete any trigger that references it, regardless of
# which space the trigger lives in. Best-effort — failures must not block the
# main delete.

if $cub space get "$PLATFORM_SPACE" --quiet &>/dev/null; then
  worker_ids=$($cub worker list --space "$PLATFORM_SPACE" \
    -o jq='.[].BridgeWorker.BridgeWorkerID' 2>/dev/null || true)
  if [[ -n "$worker_ids" ]]; then
    while IFS= read -r wid; do
      [[ -z "$wid" ]] && continue
      $cub trigger delete --space "*" \
        --where "BridgeWorkerID = '$wid'" --quiet >/dev/null 2>&1 || true
    done <<< "$worker_ids"
  fi
fi

# ── Delete the component spaces ──────────────────────────────────────────────
#
# Discover them by the Purpose label (set by setup.sh) restricted to the App
# layer so the platform space itself is not swept here.

component_spaces=$($cub space list \
  --where "Labels.Purpose = '$PURPOSE' AND Labels.Layer = 'App'" \
  --no-headers -o name 2>/dev/null || true)

if [[ -n "$component_spaces" ]]; then
  echo "Deleting component spaces..."
  while IFS= read -r sp; do
    [[ -z "$sp" ]] && continue
    sp="${sp##*/}"   # tolerate "space/slug" form from -o name
    if $cub space delete "$sp" --recursive-force; then
      echo "  ✓ deleted $sp"
    else
      echo "  ✗ could not delete $sp — investigate remaining references." >&2
    fi
  done <<< "$component_spaces"
else
  echo "No component spaces found for Purpose='$PURPOSE'."
fi

# ── Delete the platform space ────────────────────────────────────────────────

if $cub space get "$PLATFORM_SPACE" --quiet &>/dev/null; then
  echo "Deleting platform space '$PLATFORM_SPACE'..."
  if $cub space delete "$PLATFORM_SPACE" --recursive-force; then
    echo "  ✓ deleted $PLATFORM_SPACE"
  else
    echo "ERROR: Could not delete platform space '$PLATFORM_SPACE'. Investigate remaining references." >&2
    exit 1
  fi
else
  echo "Platform space '$PLATFORM_SPACE' not found — skipping."
fi

# ── Delete the local kind cluster ────────────────────────────────────────────

if command -v kind &>/dev/null && kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "Deleting kind cluster '$CLUSTER_NAME'..."
  kind delete cluster --name "$CLUSTER_NAME"
fi

echo "Done."
