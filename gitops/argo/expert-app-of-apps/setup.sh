#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
CONTROL_SPACE="${GITOPS_ARGO_EXPERT_CONTROL_SPACE:-gitops-argo-expert-control}"
DEV_SPACE="${GITOPS_ARGO_EXPERT_DEV_SPACE:-gitops-argo-expert-dev}"
STAGING_SPACE="${GITOPS_ARGO_EXPERT_STAGING_SPACE:-gitops-argo-expert-staging}"
PROD_SPACE="${GITOPS_ARGO_EXPERT_PROD_SPACE:-gitops-argo-expert-prod}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example renders an Argo CD app-of-apps repo with kustomize, once per
environment, then uploads each render into its own ConfigHub Space with
"cub variant upload". It does not create or touch a Kubernetes cluster and
it does not install or talk to Argo CD.
EOF_USAGE
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
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
This is a read-only setup plan for gitops/argo/expert-app-of-apps.
Nothing will be mutated.

Conceptual model:

  bootstrap/root-app.yaml            (applied by hand, once)
    -> bootstrap/children/
         projects.yaml               (sync wave -10, two AppProjects, one sync window)
         platform-addons-appset.yaml (sync wave 0, matrix: 3 clusters x add-ons)
         storefront-app-of-apps.yaml (sync wave 10, the child app-of-apps)
           -> apps-of-apps/storefront/
                checkout-cache.yaml  (sync wave 0, cluster generator, Helm in Kustomize)
                apptique.yaml        (sync wave 5, cluster generator)

  clusters/  three registered clusters: dev-1 (canary), staging-1
             (secondary), prod-1 (primary)

This example will:
- render the control-plane objects (bootstrap, children, child app-of-apps,
  cluster registrations) with "kustomize build"
- render apps/apptique/overlays/<env> for dev, staging and prod
- render apps/checkout-cache/overlays/<env> with "kustomize build --enable-helm"
- render apps/platform/cluster-baseline/overlays/<env>
- upload the control-plane render into ConfigHub Space "$CONTROL_SPACE"
- upload each environment's renders into "$DEV_SPACE", "$STAGING_SPACE" and
  "$PROD_SPACE" using "cub variant upload --component <app> --variant <env>"

ConfigHub mutations if you run without --explain:
- creates (or updates) Space "$CONTROL_SPACE" with the Argo control objects
- creates (or updates) Space "$DEV_SPACE" with the dev Units
- creates (or updates) Space "$STAGING_SPACE" with the staging Units
- creates (or updates) Space "$PROD_SPACE" with the prod Units

This example never creates or mutates a live Kubernetes cluster and never
talks to an Argo CD instance. The bootstrap root Application is reference
material for what a real Argo CD instance would be given.
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg controlSpace "$CONTROL_SPACE" \
    --arg devSpace "$DEV_SPACE" \
    --arg stagingSpace "$STAGING_SPACE" \
    --arg prodSpace "$PROD_SPACE" \
    '{
      example_name: "gitops-argo-expert-app-of-apps",
      mutates: false,
      mutates_confighub: true,
      mutates_live_infra: false,
      spaces: [$controlSpace, $devSpace, $stagingSpace, $prodSpace],
      units: [
        "argo-control",
        "apptique-dev", "checkout-cache-dev", "cluster-baseline-dev",
        "apptique-staging", "checkout-cache-staging", "cluster-baseline-staging",
        "apptique-prod", "checkout-cache-prod", "cluster-baseline-prod"
      ],
      apps: ["apptique", "checkout-cache", "cluster-baseline"],
      environments: ["dev", "staging", "prod"],
      clusters: ["dev-1", "staging-1", "prod-1"],
      namespaces: ["argocd", "platform-system", "storefront-dev", "storefront-staging", "storefront-prod"],
      evaluation_modes: {
        fast_preview: {
          mutates: false,
          commands: ["./setup.sh --explain", "./setup.sh --explain-json | jq"]
        },
        fast_operational_evaluation: {
          mutates_confighub: true,
          mutates_live_infra: false,
          commands: ["./setup.sh", "./verify.sh"],
          stop_before_cleanup: true
        }
      }
    }'
  exit 0
fi

require_cmd kustomize
require_cmd helm
require_cmd cub

mkdir -p "$VAR_DIR"

{
  kustomize build "$SCRIPT_DIR/bootstrap"
  echo "---"
  kustomize build "$SCRIPT_DIR/bootstrap/children"
  echo "---"
  kustomize build "$SCRIPT_DIR/apps-of-apps/storefront"
  echo "---"
  kustomize build "$SCRIPT_DIR/clusters"
} > "$VAR_DIR/rendered-control.yaml"

for env in dev staging prod; do
  kustomize build "$SCRIPT_DIR/apps/apptique/overlays/$env" > "$VAR_DIR/rendered-apptique-$env.yaml"
  kustomize build --enable-helm "$SCRIPT_DIR/apps/checkout-cache/overlays/$env" > "$VAR_DIR/rendered-checkout-cache-$env.yaml"
  kustomize build "$SCRIPT_DIR/apps/platform/cluster-baseline/overlays/$env" > "$VAR_DIR/rendered-cluster-baseline-$env.yaml"
done

cub variant upload \
  --component argo-control --variant control --environment Control \
  --space "$CONTROL_SPACE" \
  "$VAR_DIR/rendered-control.yaml"

upload_env() {
  local env="$1"
  local space="$2"
  local label="$3"
  local app
  for app in apptique checkout-cache cluster-baseline; do
    cub variant upload \
      --component "$app" --variant "$env" --environment "$label" \
      --space "$space" \
      "$VAR_DIR/rendered-$app-$env.yaml"
  done
  echo "Uploaded the $env renders to Space $space."
}

upload_env dev "$DEV_SPACE" Dev
upload_env staging "$STAGING_SPACE" Staging
upload_env prod "$PROD_SPACE" Prod

echo "Uploaded the Argo control objects to Space $CONTROL_SPACE."
echo "Next: ./verify.sh"
