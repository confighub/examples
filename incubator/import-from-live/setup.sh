#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="$SCRIPT_DIR/fixtures"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
CLUSTER_NAME="${IMPORT_FROM_LIVE_CLUSTER_NAME:-import-from-live}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example creates a local kind cluster, applies copied fixtures,
and writes a dry-run import proposal under sample-output/.
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

install_argocd_crd() {
  kubectl apply -f - <<'CRDEOF' >/dev/null
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: applications.argoproj.io
spec:
  group: argoproj.io
  names:
    kind: Application
    listKind: ApplicationList
    plural: applications
    singular: application
    shortNames:
      - app
  scope: Namespaced
  versions:
    - name: v1alpha1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          x-kubernetes-preserve-unknown-fields: true
CRDEOF
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
This is a read-only setup plan for import-from-live.
No ConfigHub state will be mutated.

If you run without --explain, this example will:
- create or replace a local kind cluster named ${CLUSTER_NAME}
- install the Argo Application CRD only
- create four namespaces
- apply copied Argo, Helm, and native fixture resources
- run cub-scout import --dry-run --json
- write local output under ${OUTPUT_DIR}
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n --arg output_dir "$OUTPUT_DIR" --arg cluster_name "$CLUSTER_NAME" '{
    example: "import-from-live",
    mutatesConfighub: false,
    mutatesLiveInfrastructure: true,
    writesLocalFilesOnly: false,
    clusterType: "kind",
    clusterName: $cluster_name,
    outputDir: $output_dir,
    requires: ["kind", "kubectl", "docker", "jq", "cub-scout"],
    source: "live-cluster",
    command: "cub-scout import --dry-run --json"
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

install_argocd_crd

for ns in argocd myapp-dev myapp-staging myapp-prod; do
  kubectl create namespace "$ns" >/dev/null 2>&1 || true
done

kubectl apply -f "$FIXTURES_DIR/applications.yaml" >/dev/null
kubectl apply -f "$FIXTURES_DIR/deployments.yaml" >/dev/null
kubectl apply -f "$FIXTURES_DIR/statefulsets.yaml" >/dev/null
kubectl apply -f "$FIXTURES_DIR/native.yaml" >/dev/null

kubectl wait --for=condition=available deployment --all -n myapp-dev --timeout=60s >/dev/null 2>&1 || true
kubectl wait --for=condition=available deployment --all -n myapp-staging --timeout=60s >/dev/null 2>&1 || true
kubectl wait --for=condition=available deployment --all -n myapp-prod --timeout=60s >/dev/null 2>&1 || true

"$CUB_SCOUT" import --dry-run > "$OUTPUT_DIR/import-preview.txt"
"$CUB_SCOUT" import --dry-run --json > "$OUTPUT_DIR/suggestion.json"
jq -S . "$OUTPUT_DIR/suggestion.json" > "$OUTPUT_DIR/suggestion.normalized.json"
kubectl get application -n argocd -o json > "$OUTPUT_DIR/applications.json"
kubectl get deployment -A -o json > "$OUTPUT_DIR/deployments.json"
kubectl get statefulset -A -o json > "$OUTPUT_DIR/statefulsets.json"

echo "Saved import preview to: $OUTPUT_DIR/import-preview.txt"
echo "Saved dry-run suggestion to: $OUTPUT_DIR/suggestion.json"
