#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${GRAPH_EXPORT_CLUSTER_NAME:-graph-export}"
VAR_DIR="$SCRIPT_DIR/var"
export KUBECONFIG="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

cluster_exists() {
  docker ps -a --format '{{.Names}}' | grep -qx "${CLUSTER_NAME}-control-plane"
}

if command -v kind >/dev/null 2>&1 && command -v docker >/dev/null 2>&1; then
  if cluster_exists; then
    kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
  fi
fi
rm -f "$SCRIPT_DIR"/sample-output/*
rm -f "$KUBECONFIG"

echo "Deleted kind cluster: $CLUSTER_NAME"
echo "Cleared local sample output"
