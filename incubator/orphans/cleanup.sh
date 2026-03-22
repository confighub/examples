#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME="${ORPHANS_CLUSTER_NAME:-orphans}"
VAR_DIR="$SCRIPT_DIR/var"
export KUBECONFIG="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
rm -f "$SCRIPT_DIR"/sample-output/*
rm -f "$KUBECONFIG"

echo "Deleted kind cluster: $CLUSTER_NAME"
echo "Cleared local sample output"
