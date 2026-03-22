#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
CLUSTER_NAME="${CUSTOM_OWNERSHIP_CLUSTER_NAME:-custom-ownership-detectors}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example creates a local kind cluster, applies native fixtures,
and writes local ownership inspection output under sample-output/.
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
This is a read-only setup plan for custom-ownership-detectors.
No ConfigHub state will be mutated.

If you run without --explain, this example will:
- create or replace a local kind cluster named ${CLUSTER_NAME}
- create the namespace detectors-demo
- apply three native Deployment fixtures
- point cub-scout at the copied detectors file
- write local ownership inspection output under ${OUTPUT_DIR}
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n --arg output_dir "$OUTPUT_DIR" --arg cluster_name "$CLUSTER_NAME" ' {
    example: "custom-ownership-detectors",
    mutatesConfighub: false,
    mutatesLiveInfrastructure: true,
    writesLocalFilesOnly: false,
    clusterType: "kind",
    clusterName: $cluster_name,
    outputDir: $output_dir,
    detectorsSource: "local-file",
    requires: ["kind", "kubectl", "docker", "jq", "cub-scout"],
    command: "cub-scout map list --json"
  }'
  exit 0
fi

preflight
CUB_SCOUT="$(resolve_cub_scout)"
mkdir -p "$OUTPUT_DIR"
rm -f "$OUTPUT_DIR"/*.json "$OUTPUT_DIR"/*.txt

kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
kind create cluster --name "$CLUSTER_NAME" --wait 60s >/dev/null
kind export kubeconfig --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

kubectl create namespace detectors-demo >/dev/null 2>&1 || true
kubectl apply -f "$FIXTURES_DIR/workloads.yaml" >/dev/null
kubectl wait --for=condition=available deployment --all -n detectors-demo --timeout=120s >/dev/null 2>&1 || true

export CUB_SCOUT_OWNERSHIP_DETECTORS="$SCRIPT_DIR/detectors.yaml"
"$CUB_SCOUT" map list --json --namespace detectors-demo > "$OUTPUT_DIR/map.json"
"$CUB_SCOUT" explain deployment/payments-api -n detectors-demo --format json > "$OUTPUT_DIR/payments-api.explain.json"
"$CUB_SCOUT" explain deployment/infra-ui -n detectors-demo --format json > "$OUTPUT_DIR/infra-ui.explain.json"
"$CUB_SCOUT" trace deployment/payments-api -n detectors-demo --format json > "$OUTPUT_DIR/payments-api.trace.json"

kubectl get deployment -n detectors-demo -o json > "$OUTPUT_DIR/deployments.json"

echo "Saved ownership inventory to: $OUTPUT_DIR/map.json"
echo "Saved explain summaries and trace result under: $OUTPUT_DIR"
