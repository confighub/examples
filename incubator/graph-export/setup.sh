#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
CLUSTER_NAME="${GRAPH_EXPORT_CLUSTER_NAME:-graph-export}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example creates a local kind cluster, applies one Deployment,
and writes graph export artifacts under sample-output/.
EOF_USAGE
}

resolve_cub_scout() {
  if [[ -n "${CUB_SCOUT_BIN:-}" && -x "${CUB_SCOUT_BIN}" ]]; then
    printf '%s\n' "$CUB_SCOUT_BIN"
    return 0
  fi

  if command -v cub-scout >/dev/null 2>&1; then
    command -v cub-scout
    return 0
  fi

  local repo_root="/Users/alexis/Public/github-repos/cub-scout"
  if [[ -x "$repo_root/cub-scout" ]]; then
    printf '%s\n' "$repo_root/cub-scout"
    return 0
  fi

  if [[ -d "$repo_root/cmd/cub-scout" ]]; then
    (cd "$repo_root" && go build -o cub-scout ./cmd/cub-scout >/dev/null)
    printf '%s\n' "$repo_root/cub-scout"
    return 0
  fi

  echo "Could not find cub-scout. Set CUB_SCOUT_BIN or install cub-scout in PATH." >&2
  return 1
}

preflight() {
  local tools=(kind kubectl docker jq)
  local tool
  for tool in "${tools[@]}"; do
    command -v "$tool" >/dev/null 2>&1 || {
      echo "Missing required tool: $tool" >&2
      exit 1
    }
  done

  if ! docker info >/dev/null 2>&1; then
    echo "Docker is not running" >&2
    exit 1
  fi
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain)
      EXPLAIN=1
      shift
      ;;
    --explain-json)
      EXPLAIN_JSON=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unexpected argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$EXPLAIN" -eq 1 ]]; then
  cat <<EOF_PLAN
This is a read-only setup plan for graph-export.
No ConfigHub state will be mutated.

If you run without --explain, this example will:
- create or replace a local kind cluster named ${CLUSTER_NAME}
- create the namespace graph-demo
- apply one Deployment fixture
- export graph artifacts in json, dot, svg, and html formats
- write local output under ${OUTPUT_DIR}
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n --arg output_dir "$OUTPUT_DIR" --arg cluster_name "$CLUSTER_NAME" '{
    example: "graph-export",
    mutatesConfighub: false,
    mutatesLiveInfrastructure: true,
    writesLocalFilesOnly: false,
    clusterType: "kind",
    clusterName: $cluster_name,
    outputDir: $output_dir,
    requires: ["kind", "kubectl", "docker", "jq", "cub-scout"],
    command: "cub-scout graph export --format json"
  }'
  exit 0
fi

preflight
CUB_SCOUT="$(resolve_cub_scout)"
mkdir -p "$OUTPUT_DIR"
rm -f "$OUTPUT_DIR"/*

kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
kind create cluster --name "$CLUSTER_NAME" --wait 60s >/dev/null
kind export kubeconfig --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

kubectl create namespace graph-demo >/dev/null 2>&1 || true
kubectl apply -f "$FIXTURES_DIR/deployment.yaml" >/dev/null
kubectl rollout status deployment/graph-app -n graph-demo --timeout=180s >/dev/null

"$CUB_SCOUT" graph export --format json --namespace graph-demo > "$OUTPUT_DIR/graph.json"
"$CUB_SCOUT" graph export --format dot --namespace graph-demo > "$OUTPUT_DIR/graph.dot"
"$CUB_SCOUT" graph export --format svg --namespace graph-demo --output "$OUTPUT_DIR/graph.svg" >/dev/null
"$CUB_SCOUT" graph export --format html --namespace graph-demo --output "$OUTPUT_DIR/graph.html" >/dev/null

kubectl get deployment -n graph-demo -o json > "$OUTPUT_DIR/deployments.json"
kubectl get replicaset -n graph-demo -o json > "$OUTPUT_DIR/replicasets.json"
kubectl get pod -n graph-demo -o json > "$OUTPUT_DIR/pods.json"

echo "Saved graph exports under: $OUTPUT_DIR"
