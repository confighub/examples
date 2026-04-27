#!/bin/bash
# ConfigHub Demo Cleanup
#
# Deletes all demo data by label. This removes all spaces (and their units,
# targets, workers, etc.) that were created by setup.sh.
#
# Usage:
#   ./cleanup.sh
#   CUB=/path/to/cub ./cleanup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/lib.sh"

echo "=== ConfigHub Demo Cleanup ==="
echo ""
echo "This will delete ALL spaces with label ExampleName=${EXAMPLE_NAME}"
echo ""

# Clear cross-space trigger references on workers physically located in our
# spaces. A trigger in another space (e.g. a different demo, or a leftover
# from a previous experiment) referencing one of these workers blocks the
# bulk space delete with "Cannot modify BridgeWorker because of an invalid
# reference from Triggers".
#
# Filter by the *parent space's* label, not the worker's own labels — workers
# installed via `cub worker install` (e.g. the kyverno-cli-worker) carry no
# labels of their own, so a worker-labels filter would miss exactly the case
# this sweep exists to handle. Failures here are non-fatal.
worker_ids=$($CUB worker list --space "*" \
  --where "Space.Labels.ExampleName = '${EXAMPLE_NAME}'" \
  -o jq='.[].BridgeWorker.BridgeWorkerID' --quiet 2>/dev/null || true)

if [[ -n "$worker_ids" ]]; then
  while IFS= read -r wid; do
    [[ -z "$wid" ]] && continue
    $CUB trigger delete --space "*" \
      --where "BridgeWorkerID = '$wid'" --quiet >/dev/null 2>&1 || true
  done <<< "$worker_ids"
fi

$CUB space delete --where "Labels.ExampleName = '${EXAMPLE_NAME}'" --recursive

echo ""
echo "=== Cleanup complete ==="
