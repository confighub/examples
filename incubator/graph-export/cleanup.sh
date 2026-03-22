#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${GRAPH_EXPORT_CLUSTER_NAME:-graph-export}"

kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
rm -f "$SCRIPT_DIR"/sample-output/*

echo "Deleted kind cluster: $CLUSTER_NAME"
echo "Cleared local sample output"
