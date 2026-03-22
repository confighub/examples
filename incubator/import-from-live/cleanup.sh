#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${IMPORT_FROM_LIVE_CLUSTER_NAME:-import-from-live}"

kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
rm -f "$SCRIPT_DIR"/sample-output/*.json "$SCRIPT_DIR"/sample-output/*.txt

echo "Deleted kind cluster: $CLUSTER_NAME"
echo "Cleared local sample output"
