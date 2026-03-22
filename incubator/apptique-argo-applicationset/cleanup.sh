#!/usr/bin/env bash
set -euo pipefail

kubectl delete applicationset apptique -n argocd --ignore-not-found
kubectl delete namespace apptique-dev apptique-prod --ignore-not-found

echo "Requested cleanup for apptique Argo ApplicationSet resources."
