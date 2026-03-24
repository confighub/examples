#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${GRAPH_EXPORT_CLUSTER_NAME:-graph-export}"
export KUBECONFIG="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

if [[ ! -f "$OUTPUT_DIR/graph.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

kubectl get deployment -n graph-demo graph-app >/dev/null
kubectl get replicaset -n graph-demo >/dev/null
kubectl get pod -n graph-demo >/dev/null

jq -e '.schema_version == "graph.v1"' "$OUTPUT_DIR/graph.json" >/dev/null
jq -e --arg cluster "kind-${CLUSTER_NAME}" '.cluster == $cluster' "$OUTPUT_DIR/graph.json" >/dev/null
jq -e '[.nodes[].kind] | index("Deployment") != null' "$OUTPUT_DIR/graph.json" >/dev/null
jq -e '[.nodes[].kind] | index("ReplicaSet") != null' "$OUTPUT_DIR/graph.json" >/dev/null
jq -e '[.nodes[].kind] | index("Pod") != null' "$OUTPUT_DIR/graph.json" >/dev/null
jq -e '[.edges[].type] | index("owns") != null' "$OUTPUT_DIR/graph.json" >/dev/null
grep -q 'digraph' "$OUTPUT_DIR/graph.dot"
grep -q '<svg' "$OUTPUT_DIR/graph.svg"
grep -qi '<!doctype html>' "$OUTPUT_DIR/graph.html"

echo "Graph export checks passed"
