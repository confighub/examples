#!/usr/bin/env bash
# cleanup.sh — Tear down the initiatives demo (component spaces + platform space)
#              and its kind cluster.
#
# The demo spans several spaces:
#   - component spaces (Labels.Purpose=<purpose>, Labels.Layer=App): aichat,
#     portal, eshop, docs, website — hold the application units.
#   - the platform space (default: initiatives-demo) — holds the bot's
#     Channel, filters, views, and triggers.
#
# Ordering matters:
#   1. Delete any Trigger — in ANY space — that runs on this demo's Channel.
#      (ConfigHub blocks Channel deletes while a Trigger references them.)
#   2. Delete the component spaces first (they reference the platform's trigger
#      filter via TriggerFilterID; clearing them before the platform avoids
#      dangling references).
#   3. Delete the platform space (units/views/filters/triggers/Channel) with
#      --recursive-force.
#   4. Delete the bot's service account.
#   5. Delete the local kind cluster created by setup.sh (best effort).
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

# ── Clear trigger references to the bot's Channel ───────────────────────────
#
# A Trigger that runs on a Channel blocks the Channel's deletion, and so the
# recursive space delete, wherever the Trigger lives. Bulk-delete every trigger
# that runs on the demo's Channel. Best-effort — failures must not block the
# main delete.

KYVERNO_BOT="$PLATFORM_SPACE-kyverno"
if $cub space get "$PLATFORM_SPACE" --quiet &>/dev/null; then
  channel_id=$($cub channel get "$PLATFORM_SPACE/$KYVERNO_BOT" \
    -o jq='.Channel.ChannelID' 2>/dev/null || true)
  if [[ -n "$channel_id" ]]; then
    $cub trigger delete \
      --where "ChannelID = '$channel_id'" --quiet >/dev/null 2>&1 || true
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

# ── Delete the bot's service account ─────────────────────────────────────────
#
# Service accounts belong to the organization, so the space delete leaves it.

$cub serviceaccount delete "$KYVERNO_BOT" --quiet >/dev/null 2>&1 || true

# ── Delete the local kind cluster ────────────────────────────────────────────

if command -v kind &>/dev/null && kind get clusters 2>/dev/null | grep -qx "$CLUSTER_NAME"; then
  echo "Deleting kind cluster '$CLUSTER_NAME'..."
  kind delete cluster --name "$CLUSTER_NAME"
fi

echo "Done."
