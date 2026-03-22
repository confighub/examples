#!/usr/bin/env bash
set -euo pipefail

kubectl delete kustomization apptique-dev apptique-prod -n flux-system --ignore-not-found
kubectl delete gitrepository apptique-examples -n flux-system --ignore-not-found
kubectl delete namespace apptique-dev apptique-prod --ignore-not-found

echo "Requested cleanup for apptique Flux monorepo resources."
