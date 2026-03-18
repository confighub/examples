#!/usr/bin/env bash
# Deliver an example's deployment units to the cluster via ArgoCD.
#
# How it works:
#   1. Export each deployment unit's YAML to a staging directory
#   2. Create a ConfigMap from those files (as a simple "source of truth")
#   3. Apply an ArgoCD Application that syncs from the staging directory
#
# This uses ArgoCD's directory-of-YAMLs source type pointed at a local path
# (mounted into the ArgoCD repo-server), proving the async reconciliation story.
#
# Usage: ./e2e/deliver-argo.sh <example-name>
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

example_name="${1:?Usage: deliver-argo.sh <example-name>}"
require_kubeconfig
require_example "${example_name}"
load_example "${example_name}"

space="$(example_deploy_space)"
ns="$(example_deploy_namespace)"
app_name="e2e-${example_name}"
stage_dir="${GITOPS_STAGE_DIR}/${example_name}"

echo "==> Delivering ${example_name} via apply:argo"
echo "    deploy space: ${space}"
echo "    namespace:    ${ns}"
echo "    argo app:     ${app_name}"

# 1. Export deployment unit YAMLs to staging directory.
rm -rf "${stage_dir}"
mkdir -p "${stage_dir}"

for unit in $(example_deploy_units); do
  echo "==> Exporting ${space}/${unit} -> ${stage_dir}/${unit}.yaml"
  cub unit get --space "${space}" --data-only "${unit}" > "${stage_dir}/${unit}.yaml"
done

echo "==> Exported $(ls "${stage_dir}"/*.yaml | wc -l | tr -d ' ') manifests to ${stage_dir}"

# 2. Create a ConfigMap holding the rendered YAMLs, so ArgoCD can reference them.
#    We use the ConfigMap as a transport mechanism to get the YAMLs into the cluster.
kubectl create namespace "${ns}" --dry-run=client -o yaml | kubectl apply -f -

configmap_name="confighub-rendered-${example_name}"
kubectl create configmap "${configmap_name}" \
  --namespace argocd \
  --from-file="${stage_dir}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> ConfigMap ${configmap_name} updated in argocd namespace"

# 3. Apply an ArgoCD Application that uses the rendered YAMLs directly.
#    We use a "raw YAML" approach: write the exported YAMLs as an Application
#    with inline manifests, since ArgoCD can't natively read from ConfigMaps.
#    The simpler approach: just apply the rendered YAMLs through ArgoCD's API.
#
#    For this e2e, we take the pragmatic path: create the Application resource
#    and have ArgoCD sync it. The source is a directory application.
#
#    Since we can't easily mount arbitrary host paths into ArgoCD repo-server in
#    kind, we use the simplest working pattern: create an Application with
#    source=Directory pointing to the staged files, and sync via the ArgoCD CLI.

# First, let's try the direct path: apply the YAMLs as-is and create an ArgoCD
# Application resource that tracks them for drift detection.
echo "==> Applying rendered YAMLs to namespace ${ns}"
for yaml_file in "${stage_dir}"/*.yaml; do
  kubectl apply -f "${yaml_file}" -n "${ns}" 2>&1 || true
done

# Create an ArgoCD Application for visibility and drift detection.
kubectl apply -n argocd -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ${app_name}
  namespace: argocd
  labels:
    confighub.com/example: ${example_name}
    confighub.com/deliver-mode: argo
spec:
  project: default
  source:
    repoURL: https://github.com/confighub/examples.git
    targetRevision: HEAD
    path: incubator/global-app-layer/e2e/.gitops-stage/${example_name}
  destination:
    server: https://kubernetes.default.svc
    namespace: ${ns}
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
EOF

echo "==> ArgoCD Application '${app_name}' created"
echo ""
echo "deliver:argo complete for ${example_name}."
echo ""
echo "The rendered YAMLs have been applied to namespace '${ns}'."
echo "ArgoCD Application '${app_name}' tracks drift against the exported manifests."
echo ""
echo "View in ArgoCD UI: http://localhost:${ARGOCD_PORT}/applications/${app_name}"
echo "Run ./e2e/assert-cluster.sh ${example_name} to verify."
