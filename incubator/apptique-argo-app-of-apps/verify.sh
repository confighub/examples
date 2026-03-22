#!/usr/bin/env bash
set -euo pipefail

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

require_cmd kubectl

kubectl get application -n argocd apptique-apps >/dev/null
kubectl get application -n argocd apptique-dev >/dev/null
kubectl get application -n argocd apptique-prod >/dev/null
kubectl get namespace apptique-dev >/dev/null
kubectl get namespace apptique-prod >/dev/null
kubectl get deployment -n apptique-dev frontend >/dev/null
kubectl get deployment -n apptique-prod frontend >/dev/null
kubectl rollout status deployment/frontend -n apptique-dev --timeout=180s >/dev/null
kubectl rollout status deployment/frontend -n apptique-prod --timeout=180s >/dev/null
kubectl get service -n apptique-dev frontend >/dev/null
kubectl get service -n apptique-prod frontend >/dev/null

if command -v cub-scout >/dev/null 2>&1; then
  cub-scout map list -q "owner=ArgoCD" >/dev/null 2>&1 || true
fi

echo "All apptique-argo-app-of-apps checks passed."
