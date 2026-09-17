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
  echo "Runs offline checks only: no cluster, no Flux, no ConfigHub calls." >&2
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
require_cmd jq

mkdir -p "$VAR_DIR"

echo "==> Checking that setup.sh --explain-json is valid JSON with the expected fields"
EXPLAIN_JSON_OUT="$("$SCRIPT_DIR/setup.sh" --explain-json)"
echo "$EXPLAIN_JSON_OUT" | jq . >/dev/null

for field in example_name mutates mutates_confighub mutates_live_infra spaces units apps environments clusters namespaces tenants promotion evaluation_modes; do
  if ! echo "$EXPLAIN_JSON_OUT" | jq -e "has(\"$field\")" >/dev/null; then
    echo "setup.sh --explain-json is missing field: $field" >&2
    exit 1
  fi
done

name="$(echo "$EXPLAIN_JSON_OUT" | jq -r '.example_name')"
if [[ "$name" != "gitops-flux-expert-fleet" ]]; then
  echo "Unexpected example_name: $name" >&2
  exit 1
fi

echo "==> Checking that setup.sh --explain runs without mutation"
"$SCRIPT_DIR/setup.sh" --explain >/dev/null

echo "==> Rendering every cluster and every layer"
for env in dev staging prod; do
  kustomize build "$SCRIPT_DIR/clusters/$env" > "$VAR_DIR/rendered-cluster-$env.yaml"
  kustomize build "$SCRIPT_DIR/infrastructure/$env" > "$VAR_DIR/rendered-infrastructure-$env.yaml"
  kustomize build "$SCRIPT_DIR/apps/$env" > "$VAR_DIR/rendered-apps-$env.yaml"
  kustomize build "$SCRIPT_DIR/tenants/$env" > "$VAR_DIR/rendered-tenants-$env.yaml"
done
kustomize build "$SCRIPT_DIR/image-automation" > "$VAR_DIR/rendered-image-automation.yaml"
kustomize build "$SCRIPT_DIR/tenants/base/team-checkout/workloads" > "$VAR_DIR/rendered-tenant-workloads.yaml"

echo "==> Checking each cluster reconciles the layers in dependency order"
for env in dev staging prod; do
  f="$VAR_DIR/rendered-cluster-$env.yaml"
  grep -q "  name: infrastructure$" "$f"
  grep -q "  name: apps$" "$f"
  grep -q "  name: tenants$" "$f"
  if ! grep -q "dependsOn:" "$f"; then
    echo "Cluster $env has no dependsOn ordering" >&2
    exit 1
  fi
  depends_count="$(grep -c "^  - name: infrastructure$" "$f")"
  if [[ "$depends_count" -lt 2 ]]; then
    echo "Expected apps and tenants on cluster $env to depend on infrastructure" >&2
    exit 1
  fi
  grep -q "path: ./gitops/flux/expert-fleet/apps/$env" "$f"
  grep -q "path: ./gitops/flux/expert-fleet/infrastructure/$env" "$f"
  grep -q "path: ./gitops/flux/expert-fleet/tenants/$env" "$f"
done

echo "==> Checking image automation runs on the dev cluster only"
grep -q "  name: image-automation$" "$VAR_DIR/rendered-cluster-dev.yaml"
for env in staging prod; do
  if grep -q "  name: image-automation$" "$VAR_DIR/rendered-cluster-$env.yaml"; then
    echo "Image automation should not run on the $env cluster" >&2
    exit 1
  fi
done

echo "==> Checking the post-build substitutions the manifests depend on"
for env in dev staging prod; do
  grep -q "postBuild:" "$VAR_DIR/rendered-cluster-$env.yaml"
  grep -q "cluster_name: $env-1" "$VAR_DIR/rendered-cluster-$env.yaml"
  grep -q "environment: $env" "$VAR_DIR/rendered-cluster-$env.yaml"
  grep -q 'value: ${cluster_name}' "$VAR_DIR/rendered-apps-$env.yaml"
done

echo "==> Checking the infrastructure layer carries both sources and the HelmRelease"
for env in dev staging prod; do
  f="$VAR_DIR/rendered-infrastructure-$env.yaml"
  grep -q "^kind: GitRepository$" "$f"
  grep -q "^kind: HelmRepository$" "$f"
  grep -q "^kind: HelmRelease$" "$f"
  grep -q "  name: platform-charts$" "$f"
  grep -q "  name: edge-router$" "$f"
done

echo "==> Checking the promotion path: dev and staging on main, prod on the production branch"
for env in dev staging; do
  if ! grep -q "    branch: main" "$VAR_DIR/rendered-infrastructure-$env.yaml"; then
    echo "Expected the $env fleet source to follow main" >&2
    exit 1
  fi
done
if ! grep -q "    branch: production" "$VAR_DIR/rendered-infrastructure-prod.yaml"; then
  echo "Expected the prod fleet source to follow the production branch" >&2
  exit 1
fi

echo "==> Checking the image tags show a promotion in flight"
dev_tag="$(grep -m1 "image: nginx:" "$VAR_DIR/rendered-apps-dev.yaml" | awk '{print $2}')"
staging_tag="$(grep -m1 "image: nginx:" "$VAR_DIR/rendered-apps-staging.yaml" | awk '{print $2}')"
prod_tag="$(grep -m1 "image: nginx:" "$VAR_DIR/rendered-apps-prod.yaml" | awk '{print $2}')"
if [[ "$dev_tag" == "$staging_tag" || "$staging_tag" == "$prod_tag" ]]; then
  echo "Expected three different image tags, got $dev_tag, $staging_tag, $prod_tag" >&2
  exit 1
fi

echo "==> Checking the image automation objects are complete and write to dev only"
grep -q "^kind: ImageRepository$" "$VAR_DIR/rendered-image-automation.yaml"
grep -q "^kind: ImagePolicy$" "$VAR_DIR/rendered-image-automation.yaml"
grep -q "^kind: ImageUpdateAutomation$" "$VAR_DIR/rendered-image-automation.yaml"
grep -q "path: ./gitops/flux/expert-fleet/apps/dev" "$VAR_DIR/rendered-image-automation.yaml"
grep -q 'imagepolicy' "$SCRIPT_DIR/apps/dev/kustomization.yaml"
for env in staging prod; do
  if grep -q 'imagepolicy' "$SCRIPT_DIR/apps/$env/kustomization.yaml"; then
    echo "The $env overlay should not carry an image policy marker" >&2
    exit 1
  fi
done

echo "==> Checking the tenant has its own namespace, ServiceAccount and Kustomization"
for env in dev staging prod; do
  f="$VAR_DIR/rendered-tenants-$env.yaml"
  grep -q "^kind: Namespace$" "$f"
  grep -q "  name: team-checkout$" "$f"
  grep -q "^kind: ServiceAccount$" "$f"
  grep -q "^kind: Kustomization$" "$f"
  if ! grep -q "  serviceAccountName: team-checkout" "$f"; then
    echo "The $env tenant Kustomization does not run as the tenant ServiceAccount" >&2
    exit 1
  fi
  if ! grep -q "  targetNamespace: team-checkout" "$f"; then
    echo "The $env tenant Kustomization does not target the tenant namespace" >&2
    exit 1
  fi
done

echo "==> Checking the tenant's own workloads render"
grep -q "  name: checkout-api$" "$VAR_DIR/rendered-tenant-workloads.yaml"

echo "==> Checking every path a Kustomization points at exists in this repo"
while IFS= read -r p; do
  rel="${p#./gitops/flux/expert-fleet/}"
  if [[ ! -f "$SCRIPT_DIR/$rel/kustomization.yaml" ]]; then
    echo "Kustomization path $p has no kustomization.yaml" >&2
    exit 1
  fi
done < <(grep -rho "path: \./gitops/flux/expert-fleet/[a-z/-]*" "$SCRIPT_DIR/clusters" "$SCRIPT_DIR/image-automation" "$SCRIPT_DIR/tenants/base" | awk '{print $2}' | sort -u)

echo "All gitops-flux-expert-fleet checks passed."
