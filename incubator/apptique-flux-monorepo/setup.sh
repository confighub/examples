#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${APPTIQUE_FLUX_CLUSTER_NAME:-apptique-flux-monorepo}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"
EXAMPLES_GIT_REVISION="${EXAMPLES_GIT_REVISION:-main}"
WITH_PROD=0
EXPLAIN=0
EXPLAIN_JSON=0

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh [--with-prod]

This example creates its own local kind cluster and installs Flux with a dedicated kubeconfig under var/.
EOF_USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-prod)
      WITH_PROD=1
      shift
      ;;
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
This is a read-only setup plan for apptique-flux-monorepo.
Nothing will be mutated.

This example will:
- create a local kind cluster named $CLUSTER_NAME
- install Flux source-controller and kustomize-controller
- use a dedicated kubeconfig at $KUBECONFIG_PATH

It will apply:
- infrastructure/gitrepository.yaml
- clusters/dev/kustomization.yaml
$( [[ "$WITH_PROD" -eq 1 ]] && echo '- clusters/prod/kustomization.yaml' )

Live mutations if you run without --explain:
- local kind cluster and kubeconfig
- Flux GitRepository in flux-system
- Flux Kustomization for apptique-dev
$( [[ "$WITH_PROD" -eq 1 ]] && echo '- Flux Kustomization for apptique-prod' )
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg example "apptique-flux-monorepo" \
    --arg clusterName "$CLUSTER_NAME" \
    --arg kubeconfigPath "$KUBECONFIG_PATH" \
    --argjson withProd "$WITH_PROD" \
    '{
      example: $example,
      mutatesConfighub: false,
      mutatesLiveInfrastructure: true,
      requires: ["kubectl", "flux", "kind"],
      clusterType: "kind",
      clusterName: $clusterName,
      kubeconfigPath: $kubeconfigPath,
      examplesGitRevision: $examplesGitRevision,
      fluxInstalledBySetup: true,
      applies: ([
        "infrastructure/gitrepository.yaml",
        "clusters/dev/kustomization.yaml"
      ] + (if $withProd == 1 then ["clusters/prod/kustomization.yaml"] else [] end)),
      expectedNamespaces: (["apptique-dev"] + (if $withProd == 1 then ["apptique-prod"] else [] end))
    }' \
    --arg examplesGitRevision "$EXAMPLES_GIT_REVISION"
  exit 0
fi

require_cmd kubectl
require_cmd flux
require_cmd kind

mkdir -p "$VAR_DIR"
export KUBECONFIG="$KUBECONFIG_PATH"

if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
fi

kind create cluster --name "$CLUSTER_NAME" --wait 60s --kubeconfig "$KUBECONFIG_PATH" >/dev/null
kubectl config use-context "kind-$CLUSTER_NAME" >/dev/null

MANIFEST_FILE="$(mktemp)"
trap 'rm -f "$MANIFEST_FILE"' EXIT

flux install \
  --components="source-controller,kustomize-controller" \
  --network-policy=false \
  --export > "$MANIFEST_FILE"
kubectl apply -f "$MANIFEST_FILE" >/dev/null
kubectl -n flux-system wait --for=condition=available deployment/source-controller --timeout=600s >/dev/null
kubectl -n flux-system wait --for=condition=available deployment/kustomize-controller --timeout=600s >/dev/null
kubectl wait --for=condition=Established --timeout=120s \
  crd/gitrepositories.source.toolkit.fluxcd.io \
  crd/kustomizations.kustomize.toolkit.fluxcd.io >/dev/null

sed "s/branch: main/branch: $EXAMPLES_GIT_REVISION/" "$SCRIPT_DIR/infrastructure/gitrepository.yaml" | kubectl apply -f - >/dev/null
kubectl apply -f "$SCRIPT_DIR/clusters/dev/kustomization.yaml" >/dev/null
if [[ "$WITH_PROD" -eq 1 ]]; then
  kubectl apply -f "$SCRIPT_DIR/clusters/prod/kustomization.yaml" >/dev/null
fi

kubectl wait --for=condition=ready gitrepository/apptique-examples -n flux-system --timeout=240s >/dev/null
kubectl wait --for=condition=ready kustomization/apptique-dev -n flux-system --timeout=600s >/dev/null
kubectl rollout status deployment/frontend -n apptique-dev --timeout=240s >/dev/null

if [[ "$WITH_PROD" -eq 1 ]]; then
  kubectl wait --for=condition=ready kustomization/apptique-prod -n flux-system --timeout=600s >/dev/null
  kubectl rollout status deployment/frontend -n apptique-prod --timeout=240s >/dev/null
fi

echo "Applied apptique Flux monorepo resources."
echo "Next: ./verify.sh$( [[ "$WITH_PROD" -eq 1 ]] && echo ' --with-prod' )"
