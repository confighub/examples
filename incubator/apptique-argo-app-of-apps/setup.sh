#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CLUSTER_NAME="${APPTIQUE_ARGO_APPOFAPPS_CLUSTER_NAME:-apptique-argo-app-of-apps}"
KUBECONFIG_PATH="$VAR_DIR/$CLUSTER_NAME.kubeconfig"
EXAMPLES_GIT_REVISION="${EXAMPLES_GIT_REVISION:-main}"
ESCAPED_GIT_REVISION="${EXAMPLES_GIT_REVISION//\//\\/}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example creates its own local kind cluster and installs Argo CD with a dedicated kubeconfig under var/.
EOF_USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

wait_for_resource() {
  local description="$1"
  shift
  local attempts="${ATTEMPTS:-120}"
  local sleep_seconds="${SLEEP_SECONDS:-5}"
  local i
  for ((i=1; i<=attempts; i++)); do
    if "$@" >/dev/null 2>&1; then
      return 0
    fi
    sleep "$sleep_seconds"
  done
  echo "Timed out waiting for $description" >&2
  return 1
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
This is a read-only setup plan for apptique-argo-app-of-apps.
Nothing will be mutated.

This example will:
- create a local kind cluster named $CLUSTER_NAME
- install Argo CD with a dedicated kubeconfig at $KUBECONFIG_PATH
- apply a rendered root Application pointing at examples revision $EXAMPLES_GIT_REVISION
- let the root app create child Applications for dev and prod
- if the revision is not main, patch the child Applications to sync that branch for pre-merge validation

Live mutations if you run without --explain:
- local kind cluster and kubeconfig
- Argo CD in argocd
- one root Argo Application in argocd
- child Applications for dev and prod
- the resulting namespaces, deployment, and service
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg clusterName "$CLUSTER_NAME" \
    --arg kubeconfigPath "$KUBECONFIG_PATH" \
    --arg revision "$EXAMPLES_GIT_REVISION" \
    '{
      example: "apptique-argo-app-of-apps",
      mutatesConfighub: false,
      mutatesLiveInfrastructure: true,
      requires: ["kubectl", "kind"],
      clusterType: "kind",
      clusterName: $clusterName,
      kubeconfigPath: $kubeconfigPath,
      argoInstalledBySetup: true,
      examplesGitRevision: $revision,
      applies: ["root/root-app.yaml"],
      expectedApplications: ["apptique-apps", "apptique-dev", "apptique-prod"],
      expectedNamespaces: ["apptique-dev", "apptique-prod"]
    }'
  exit 0
fi

require_cmd kubectl
require_cmd kind
mkdir -p "$VAR_DIR"
export KUBECONFIG="$KUBECONFIG_PATH"

if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME" >/dev/null 2>&1 || true
fi
kind create cluster --name "$CLUSTER_NAME" --wait 60s --kubeconfig "$KUBECONFIG_PATH" >/dev/null
kubectl config use-context "kind-$CLUSTER_NAME" >/dev/null

kubectl create namespace argocd --dry-run=client -o yaml | kubectl apply -f - >/dev/null
kubectl apply --server-side -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml >/dev/null
kubectl wait --for=condition=Available --timeout=600s \
  -n argocd deployment/argocd-server \
  deployment/argocd-repo-server \
  deployment/argocd-applicationset-controller \
  deployment/argocd-redis >/dev/null
kubectl wait --for=condition=Ready --timeout=600s -n argocd pod/argocd-application-controller-0 >/dev/null

sed \
  -e "s/targetRevision: main/targetRevision: $ESCAPED_GIT_REVISION/" \
  -e 's/selfHeal: true/selfHeal: false/' \
  "$SCRIPT_DIR/root/root-app.yaml" | kubectl apply -f - >/dev/null

wait_for_resource "Application/apptique-apps" kubectl get application -n argocd apptique-apps
wait_for_resource "Application/apptique-dev" kubectl get application -n argocd apptique-dev
wait_for_resource "Application/apptique-prod" kubectl get application -n argocd apptique-prod

if [[ "$EXAMPLES_GIT_REVISION" != "main" ]]; then
  kubectl patch application apptique-dev -n argocd --type merge -p "{\"spec\":{\"source\":{\"targetRevision\":\"$EXAMPLES_GIT_REVISION\"}}}" >/dev/null
  kubectl patch application apptique-prod -n argocd --type merge -p "{\"spec\":{\"source\":{\"targetRevision\":\"$EXAMPLES_GIT_REVISION\"}}}" >/dev/null
fi

wait_for_resource "Namespace/apptique-dev" kubectl get namespace apptique-dev
wait_for_resource "Namespace/apptique-prod" kubectl get namespace apptique-prod
kubectl rollout status deployment/frontend -n apptique-dev --timeout=300s >/dev/null
kubectl rollout status deployment/frontend -n apptique-prod --timeout=300s >/dev/null
kubectl get service -n apptique-dev frontend >/dev/null
kubectl get service -n apptique-prod frontend >/dev/null

echo "Applied apptique Argo app-of-apps resources."
echo "Next: ./verify.sh"
