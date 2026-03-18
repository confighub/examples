#!/usr/bin/env bash
# Deliver an example's deployment units through the current Argo-oriented e2e path.
#
# Important: this helper is intentionally described as "Argo-oriented" rather than
# "pure GitOps" because it does a hybrid flow:
#   1. Export each deployment unit's YAML to a local staging directory
#   2. Create a ConfigMap from those files for visibility inside the cluster
#   3. Apply the rendered YAMLs directly with kubectl
#   4. Create an ArgoCD Application for visibility and drift detection
#
# What this proves today:
#   - ConfigHub can render/export the deployment units deterministically
#   - the rendered manifests can be staged for an Argo-shaped workflow
#   - ArgoCD can track the staged app for visibility/drift detection
#
# What this does not prove today:
#   - pure Argo reconciliation from Git
#   - async controller-driven delivery as the sole deployment mechanism
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

echo "==> Delivering ${example_name} via hybrid apply:argo path"
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

# 2. Create a ConfigMap holding the rendered YAMLs for visibility inside the cluster.
#    This is useful evidence for the staged output, but it is not the deploy source.
kubectl create namespace "${ns}" --dry-run=client -o yaml | kubectl apply -f -

configmap_name="confighub-rendered-${example_name}"
kubectl create configmap "${configmap_name}" \
  --namespace argocd \
  --from-file="${stage_dir}" \
  --dry-run=client -o yaml | kubectl apply -f -

echo "==> ConfigMap ${configmap_name} updated in argocd namespace"

# 3. Apply the rendered YAMLs directly, then create an ArgoCD Application that
#    points at the staged path for visibility/drift detection.
#
#    This is a pragmatic hybrid path for local e2e work. The live resources below
#    are created by kubectl apply, not by Argo reconciliation from Git.
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

echo "==> ArgoCD Application '${app_name}' created for visibility/drift detection"
echo ""
echo "deliver:argo complete for ${example_name}."
echo ""
echo "The rendered YAMLs have been applied directly to namespace '${ns}'."
echo "ArgoCD Application '${app_name}' tracks drift/visibility against the staged manifests."
echo "This helper is a hybrid Argo-oriented proof, not a pure Argo reconciliation proof."
echo ""
echo "View in ArgoCD UI: http://localhost:${ARGOCD_PORT}/applications/${app_name}"
echo "Run ./e2e/assert-cluster.sh ${example_name} to verify."
