#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURE="$SCRIPT_DIR/fixtures/orphans.yaml"
FLUX_FIXTURE="$SCRIPT_DIR/fixtures/flux/podinfo-kustomizations.yaml"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${PLATFORM_EXAMPLE_CLUSTER_NAME:-platform-example}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"
EXPLAIN=0
EXPLAIN_JSON=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain) EXPLAIN=1 ;;
    --explain-json) EXPLAIN_JSON=1 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
  shift
done

if [[ $EXPLAIN -eq 1 ]]; then
  cat <<TEXT
This example will:
- create a local kind cluster named $CLUSTER_NAME
- install Flux controllers
- apply a working Flux podinfo GitRepository and Kustomization
- apply fixtures/orphans.yaml
- capture map list, orphan inventory, and one Flux trace under sample-output/
- not write ConfigHub state
TEXT
  exit 0
fi

if [[ $EXPLAIN_JSON -eq 1 ]]; then
  jq -n --arg clusterName "$CLUSTER_NAME" '{example:"platform-example", mutatesConfighub:false, mutatesLiveInfrastructure:true, clusterType:"kind", clusterName:$clusterName}'
  exit 0
fi

command -v kind >/dev/null
command -v kubectl >/dev/null
command -v flux >/dev/null
command -v cub-scout >/dev/null
mkdir -p "$VAR_DIR" "$OUTPUT_DIR"
: > "$OUTPUT_DIR/map-list.json"
: > "$OUTPUT_DIR/orphans.json"
: > "$OUTPUT_DIR/trace-podinfo.json"
: > "$OUTPUT_DIR/flux-status.txt"

export KUBECONFIG="$KUBECONFIG_PATH"
if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
fi
kind create cluster --name "$CLUSTER_NAME" --wait 60s --kubeconfig "$KUBECONFIG_PATH" >/dev/null
kubectl config use-context "kind-$CLUSTER_NAME" >/dev/null

MANIFEST_FILE="$(mktemp)"
trap 'rm -f "$MANIFEST_FILE"' EXIT
flux install --components="source-controller,kustomize-controller" --network-policy=false --export > "$MANIFEST_FILE"
while IFS= read -r image; do
  [[ -n "$image" ]] || continue
  if ! docker image inspect "$image" >/dev/null 2>&1; then
    docker pull "$image" >/dev/null
  fi
  if ! kind load docker-image --name "$CLUSTER_NAME" "$image" >/dev/null 2>&1; then
    echo "Warning: could not preload $image into kind; continuing with cluster-side pulls" >&2
  fi
done < <(awk '/image:/ {print $2}' "$MANIFEST_FILE" | sort -u)
kubectl apply -f "$MANIFEST_FILE" >/dev/null
kubectl -n flux-system wait --for=condition=available deployment/source-controller --timeout=360s >/dev/null
kubectl -n flux-system wait --for=condition=available deployment/kustomize-controller --timeout=360s >/dev/null
kubectl wait --for=condition=Established --timeout=120s \
  crd/gitrepositories.source.toolkit.fluxcd.io \
  crd/kustomizations.kustomize.toolkit.fluxcd.io >/dev/null

kubectl apply -f "$FLUX_FIXTURE" >/dev/null
kubectl wait --for=condition=ready gitrepository/podinfo -n flux-system --timeout=240s >/dev/null
kubectl wait --for=condition=ready kustomization/podinfo -n flux-system --timeout=360s >/dev/null

kubectl apply -f "$FIXTURE" >/dev/null
kubectl wait --for=condition=available deployment/debug-nginx -n default --timeout=180s >/dev/null

flux get all -A > "$OUTPUT_DIR/flux-status.txt"
cub-scout map list --json > "$OUTPUT_DIR/map-list.json"
cub-scout map orphans --json > "$OUTPUT_DIR/orphans.json"
cub-scout trace deployment/podinfo -n podinfo --format json > "$OUTPUT_DIR/trace-podinfo.json"

echo "Saved platform outputs to: $OUTPUT_DIR"
