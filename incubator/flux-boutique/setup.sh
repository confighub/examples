#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURE="$SCRIPT_DIR/fixtures/boutique.yaml"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
VAR_DIR="$SCRIPT_DIR/var"
EXPLAIN=0
EXPLAIN_JSON=0
CLUSTER_NAME="${FLUX_BOUTIQUE_CLUSTER_NAME:-flux-boutique}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"

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
- apply fixtures/boutique.yaml
- wait for boutique Kustomizations and deployments
- capture cub-scout ownership evidence under sample-output/
- not write ConfigHub state
TEXT
  exit 0
fi

if [[ $EXPLAIN_JSON -eq 1 ]]; then
  jq -n --arg clusterName "$CLUSTER_NAME" '{example:"flux-boutique", mutatesConfighub:false, mutatesLiveInfrastructure:true, clusterType:"kind", clusterName:$clusterName}'
  exit 0
fi

command -v kind >/dev/null
command -v kubectl >/dev/null
command -v flux >/dev/null
command -v cub-scout >/dev/null
mkdir -p "$VAR_DIR"
mkdir -p "$OUTPUT_DIR"
: > "$OUTPUT_DIR/map-list.json"
: > "$OUTPUT_DIR/trace-payment.json"
: > "$OUTPUT_DIR/flux-kustomizations.txt"

export KUBECONFIG="$KUBECONFIG_PATH"
if ! kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind create cluster --name "$CLUSTER_NAME" --kubeconfig "$KUBECONFIG_PATH"
fi
kubectl config use-context "kind-$CLUSTER_NAME" >/dev/null

MANIFEST_FILE="$(mktemp)"
trap 'rm -f "$MANIFEST_FILE"' EXIT
flux install --components="source-controller,kustomize-controller" --network-policy=false --export > "$MANIFEST_FILE"
kubectl apply -f "$MANIFEST_FILE" >/dev/null
kubectl -n flux-system wait --for=condition=available deployment/source-controller --timeout=180s >/dev/null
kubectl -n flux-system wait --for=condition=available deployment/kustomize-controller --timeout=180s >/dev/null
kubectl wait --for=condition=Established --timeout=120s crd/gitrepositories.source.toolkit.fluxcd.io crd/kustomizations.kustomize.toolkit.fluxcd.io >/dev/null

kubectl apply -f "$FIXTURE" >/dev/null
kubectl wait --for=condition=ready gitrepository/boutique -n boutique --timeout=180s >/dev/null
for name in frontend cart checkout payment shipping; do
  kubectl wait --for=condition=ready "kustomization/$name" -n boutique --timeout=180s >/dev/null
done
for name in frontend cart checkout payment shipping; do
  until kubectl get deployment "$name" -n boutique >/dev/null 2>&1; do
    sleep 2
  done
done
flux get kustomizations -n boutique > "$OUTPUT_DIR/flux-kustomizations.txt"
cub-scout map list --namespace boutique --json > "$OUTPUT_DIR/map-list.json"
cub-scout trace deployment/payment -n boutique --format json > "$OUTPUT_DIR/trace-payment.json"

echo "Saved boutique outputs to: $OUTPUT_DIR"
