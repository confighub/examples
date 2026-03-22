#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${ORPHANS_CLUSTER_NAME:-orphans}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example creates a local kind cluster, applies orphan fixtures,
and writes local orphan inventory output under sample-output/.
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
This is a read-only setup plan for orphans.
No ConfigHub state will be mutated.

If you run without --explain, this example will:
- create or replace a local kind cluster named ${CLUSTER_NAME}
- apply the copied orphan fixture set
- run cub-scout map orphans --json
- capture one native trace result for deployment/debug-nginx
- write local output under ${OUTPUT_DIR}
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n --arg output_dir "$OUTPUT_DIR" --arg cluster_name "$CLUSTER_NAME" '{
    example: "orphans",
    mutatesConfighub: false,
    mutatesLiveInfrastructure: true,
    writesLocalFilesOnly: false,
    clusterType: "kind",
    clusterName: $cluster_name,
    outputDir: $output_dir,
    requires: ["kind", "kubectl", "docker", "jq", "cub-scout"],
    command: "cub-scout map orphans --json"
  }'
  exit 0
fi

preflight
CUB_SCOUT="$(resolve_cub_scout)"
mkdir -p "$VAR_DIR"
mkdir -p "$OUTPUT_DIR"
rm -f "$OUTPUT_DIR"/*

export KUBECONFIG="$KUBECONFIG_PATH"
kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
kind create cluster --name "$CLUSTER_NAME" --wait 60s --kubeconfig "$KUBECONFIG_PATH" >/dev/null
kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null

kubectl apply -f "$FIXTURES_DIR/realistic-orphans.yaml" >/dev/null
kubectl rollout status deployment/legacy-prometheus -n legacy-apps --timeout=180s >/dev/null
kubectl rollout status deployment/debug-nginx -n temp-testing --timeout=180s >/dev/null
kubectl rollout status deployment/debug-busybox -n temp-testing --timeout=180s >/dev/null
kubectl rollout status deployment/hotfix-worker -n default --timeout=180s >/dev/null

"$CUB_SCOUT" map orphans --json > "$OUTPUT_DIR/orphans.json"
set +e
"$CUB_SCOUT" trace deployment/debug-nginx -n temp-testing --format json > "$OUTPUT_DIR/debug-nginx.trace.json" 2> "$OUTPUT_DIR/debug-nginx.trace.stderr.txt"
trace_status=$?
set -e
printf '%s\n' "$trace_status" > "$OUTPUT_DIR/debug-nginx.trace.exitcode"

echo "Saved orphan inventory to: $OUTPUT_DIR/orphans.json"
echo "Saved trace output to: $OUTPUT_DIR/debug-nginx.trace.json"
