#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

usage() {
  echo "Usage: ./verify.sh" >&2
  echo "Runs offline checks only: no cluster, no Argo CD, no ConfigHub calls." >&2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unexpected argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

require_cmd kustomize
require_cmd helm
require_cmd jq

mkdir -p "$VAR_DIR"

echo "==> Checking that setup.sh --explain-json is valid JSON with the expected fields"
EXPLAIN_JSON_OUT="$("$SCRIPT_DIR/setup.sh" --explain-json)"
echo "$EXPLAIN_JSON_OUT" | jq . >/dev/null

for field in example_name mutates mutates_confighub mutates_live_infra spaces units apps environments clusters namespaces evaluation_modes; do
  if ! echo "$EXPLAIN_JSON_OUT" | jq -e "has(\"$field\")" >/dev/null; then
    echo "setup.sh --explain-json is missing field: $field" >&2
    exit 1
  fi
done

name="$(echo "$EXPLAIN_JSON_OUT" | jq -r '.example_name')"
if [[ "$name" != "gitops-argo-expert-app-of-apps" ]]; then
  echo "Unexpected example_name: $name" >&2
  exit 1
fi

echo "==> Checking that setup.sh --explain runs without mutation"
"$SCRIPT_DIR/setup.sh" --explain >/dev/null

echo "==> Rendering the Argo control objects (root app, children, child app-of-apps, clusters)"
kustomize build "$SCRIPT_DIR/bootstrap" > "$VAR_DIR/rendered-root-app.yaml"
kustomize build "$SCRIPT_DIR/bootstrap/children" > "$VAR_DIR/rendered-children.yaml"
kustomize build "$SCRIPT_DIR/apps-of-apps/storefront" > "$VAR_DIR/rendered-storefront.yaml"
kustomize build "$SCRIPT_DIR/clusters" > "$VAR_DIR/rendered-clusters.yaml"

echo "==> Rendering the vendored Helm chart on its own with helm template"
helm template checkout-cache "$SCRIPT_DIR/apps/checkout-cache/base/charts/checkout-cache" > "$VAR_DIR/rendered-chart.yaml"
grep -q "^kind: Deployment$" "$VAR_DIR/rendered-chart.yaml"
grep -q "image: \"redis:7.4-alpine\"" "$VAR_DIR/rendered-chart.yaml"

echo "==> Rendering every app overlay for dev, staging and prod"
for env in dev staging prod; do
  kustomize build "$SCRIPT_DIR/apps/apptique/overlays/$env" > "$VAR_DIR/rendered-apptique-$env.yaml"
  kustomize build --enable-helm "$SCRIPT_DIR/apps/checkout-cache/overlays/$env" > "$VAR_DIR/rendered-checkout-cache-$env.yaml"
  kustomize build "$SCRIPT_DIR/apps/platform/cluster-baseline/overlays/$env" > "$VAR_DIR/rendered-cluster-baseline-$env.yaml"
done

echo "==> Checking each environment renders the storefront app into its own namespace"
for env in dev staging prod; do
  grep -q "^kind: Namespace$" "$VAR_DIR/rendered-apptique-$env.yaml"
  grep -q "  name: storefront-$env$" "$VAR_DIR/rendered-apptique-$env.yaml"
  grep -q "  namespace: storefront-$env$" "$VAR_DIR/rendered-apptique-$env.yaml"
  grep -q "  namespace: storefront-$env$" "$VAR_DIR/rendered-checkout-cache-$env.yaml"
  grep -q "  namespace: platform-system$" "$VAR_DIR/rendered-cluster-baseline-$env.yaml"
done

echo "==> Checking the Helm-in-Kustomize app inflated the chart, not just a HelmRelease"
for env in dev staging prod; do
  grep -q "image: redis:7.4-alpine" "$VAR_DIR/rendered-checkout-cache-$env.yaml"
  grep -q "^kind: Deployment$" "$VAR_DIR/rendered-checkout-cache-$env.yaml"
  grep -q "^kind: Service$" "$VAR_DIR/rendered-checkout-cache-$env.yaml"
done

replicas_of() {
  awk '/^kind: Deployment$/{f=1} f && /^  replicas:/{print $2; exit}' "$1"
}

echo "==> Checking the environments really differ"
dev_replicas="$(replicas_of "$VAR_DIR/rendered-apptique-dev.yaml")"
prod_replicas="$(replicas_of "$VAR_DIR/rendered-apptique-prod.yaml")"
if [[ "$dev_replicas" == "$prod_replicas" ]]; then
  echo "Expected dev and prod apptique replica counts to differ, both were $dev_replicas" >&2
  exit 1
fi

echo "==> Checking prod is one image tag behind dev, the rollout this repo shows mid-flight"
dev_image="$(grep -m1 "image: nginx:" "$VAR_DIR/rendered-apptique-dev.yaml" | awk '{print $2}')"
prod_image="$(grep -m1 "image: nginx:" "$VAR_DIR/rendered-apptique-prod.yaml" | awk '{print $2}')"
if [[ "$dev_image" == "$prod_image" ]]; then
  echo "Expected the dev and prod apptique image tags to differ, both were $dev_image" >&2
  exit 1
fi

echo "==> Checking the root Application points at the children directory in this example"
grep -q "path: gitops/argo/expert-app-of-apps/bootstrap/children" "$VAR_DIR/rendered-root-app.yaml"

echo "==> Checking the sync waves order projects, then platform add-ons, then the storefront child"
grep -q 'argocd.argoproj.io/sync-wave: "-10"' "$VAR_DIR/rendered-children.yaml"
grep -q 'argocd.argoproj.io/sync-wave: "0"' "$VAR_DIR/rendered-children.yaml"
grep -q 'argocd.argoproj.io/sync-wave: "10"' "$VAR_DIR/rendered-children.yaml"
grep -q 'argocd.argoproj.io/sync-wave: "5"' "$VAR_DIR/rendered-storefront.yaml"

echo "==> Checking the production sync window exists and denies working-hours syncs"
grep -q "syncWindows:" "$VAR_DIR/rendered-children.yaml"
grep -q "kind: deny" "$VAR_DIR/rendered-children.yaml"
grep -q "manualSync: false" "$VAR_DIR/rendered-children.yaml"

echo "==> Checking the generators fail loudly on a missing cluster label"
for f in "$VAR_DIR/rendered-children.yaml" "$VAR_DIR/rendered-storefront.yaml"; do
  grep -q "missingkey=error" "$f"
done

echo "==> Checking three clusters are registered with distinct rollout phases"
for phase in canary secondary primary; do
  grep -q "rollout-phase: $phase" "$VAR_DIR/rendered-clusters.yaml"
done
cluster_count="$(grep -c "argocd.argoproj.io/secret-type: cluster" "$VAR_DIR/rendered-clusters.yaml")"
if [[ "$cluster_count" -ne 3 ]]; then
  echo "Expected 3 registered clusters, found $cluster_count" >&2
  exit 1
fi

echo "==> Checking every generated path exists in this repo"
for app_path in apps/apptique/overlays apps/checkout-cache/overlays apps/platform/cluster-baseline/overlays; do
  for env in dev staging prod; do
    if [[ ! -f "$SCRIPT_DIR/$app_path/$env/kustomization.yaml" ]]; then
      echo "Generator path $app_path/$env has no kustomization.yaml" >&2
      exit 1
    fi
  done
done
grep -q "gitops/argo/expert-app-of-apps/apps/apptique/overlays" "$VAR_DIR/rendered-storefront.yaml"
grep -q "gitops/argo/expert-app-of-apps/apps/checkout-cache/overlays" "$VAR_DIR/rendered-storefront.yaml"
grep -q "gitops/argo/expert-app-of-apps/apps/platform/" "$VAR_DIR/rendered-children.yaml"
grep -q "gitops/argo/expert-app-of-apps/apps-of-apps/storefront" "$VAR_DIR/rendered-children.yaml"

echo "All gitops-argo-expert-app-of-apps checks passed."
