#!/usr/bin/env bash
set -euo pipefail

kubectl delete application apptique-apps -n argocd --ignore-not-found
kubectl delete namespace apptique-dev apptique-prod --ignore-not-found

echo "Requested cleanup for apptique Argo app-of-apps resources."
