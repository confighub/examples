#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
FLEET_SPACE="${GITOPS_FLUX_EXPERT_FLEET_SPACE:-gitops-flux-expert-fleet}"
DEV_SPACE="${GITOPS_FLUX_EXPERT_DEV_SPACE:-gitops-flux-expert-dev}"
STAGING_SPACE="${GITOPS_FLUX_EXPERT_STAGING_SPACE:-gitops-flux-expert-staging}"
PROD_SPACE="${GITOPS_FLUX_EXPERT_PROD_SPACE:-gitops-flux-expert-prod}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example renders a Flux fleet repo with kustomize, once per cluster and
once per layer, then uploads each render into a ConfigHub Space with
"cub variant upload". It does not bootstrap Flux and does not touch a
Kubernetes cluster.
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
This is a read-only setup plan for gitops/flux/expert-fleet.
Nothing will be mutated.

Conceptual model (three clusters, four layers, one source):

  clusters/<env>/
    infrastructure.yaml   -> infrastructure/<env>  (sources, HelmRelease)
    apps.yaml             -> apps/<env>            (depends on infrastructure)
    tenants.yaml          -> tenants/<env>         (depends on infrastructure)
    image-automation.yaml -> image-automation      (dev cluster only, depends on apps)

  Promotion: dev and staging read branch main. Production reads branch
  production, because infrastructure/prod patches the GitRepository. Image
  automation writes new tags into apps/dev only.

This example will:
- render clusters/<env> for dev, staging and prod with "kustomize build"
- render infrastructure/<env>, apps/<env> and tenants/<env> for each
- render image-automation and the tenant's own workloads
- upload the fleet control objects into ConfigHub Space "$FLEET_SPACE"
- upload each environment's renders into "$DEV_SPACE", "$STAGING_SPACE"
  and "$PROD_SPACE" using "cub variant upload --component <layer> --variant <env>"

ConfigHub mutations if you run without --explain:
- creates (or updates) Space "$FLEET_SPACE" with the Flux control objects
- creates (or updates) Space "$DEV_SPACE" with the dev Units
- creates (or updates) Space "$STAGING_SPACE" with the staging Units
- creates (or updates) Space "$PROD_SPACE" with the prod Units

This example never runs "flux bootstrap", never reconciles anything, and
never creates or mutates a live Kubernetes cluster.
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg fleetSpace "$FLEET_SPACE" \
    --arg devSpace "$DEV_SPACE" \
    --arg stagingSpace "$STAGING_SPACE" \
    --arg prodSpace "$PROD_SPACE" \
    '{
      example_name: "gitops-flux-expert-fleet",
      mutates: false,
      mutates_confighub: true,
      mutates_live_infra: false,
      spaces: [$fleetSpace, $devSpace, $stagingSpace, $prodSpace],
      units: [
        "flux-control",
        "infrastructure-dev", "apps-dev", "tenants-dev",
        "infrastructure-staging", "apps-staging", "tenants-staging",
        "infrastructure-prod", "apps-prod", "tenants-prod"
      ],
      apps: ["apptique", "edge-router", "checkout-api"],
      environments: ["dev", "staging", "prod"],
      clusters: ["dev-1", "staging-1", "prod-1"],
      namespaces: ["flux-system", "ingress-system", "apptique-dev", "apptique-staging", "apptique-prod", "team-checkout"],
      tenants: ["team-checkout"],
      promotion: {
        dev: "branch main, image tag written by automation",
        staging: "branch main, image tag moved by pull request",
        prod: "branch production, reached by merge"
      },
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
require_cmd cub

mkdir -p "$VAR_DIR"

{
  for env in dev staging prod; do
    kustomize build "$SCRIPT_DIR/clusters/$env"
    echo "---"
  done
  kustomize build "$SCRIPT_DIR/image-automation"
} > "$VAR_DIR/rendered-control.yaml"

for env in dev staging prod; do
  kustomize build "$SCRIPT_DIR/infrastructure/$env" > "$VAR_DIR/rendered-infrastructure-$env.yaml"
  kustomize build "$SCRIPT_DIR/apps/$env" > "$VAR_DIR/rendered-apps-$env.yaml"
  kustomize build "$SCRIPT_DIR/tenants/$env" > "$VAR_DIR/rendered-tenants-$env.yaml"
done

cub variant upload \
  --component flux-control --variant fleet --environment Control \
  --space "$FLEET_SPACE" \
  "$VAR_DIR/rendered-control.yaml"

upload_env() {
  local env="$1"
  local space="$2"
  local label="$3"
  local layer
  for layer in infrastructure apps tenants; do
    cub variant upload \
      --component "$layer" --variant "$env" --environment "$label" \
      --space "$space" \
      "$VAR_DIR/rendered-$layer-$env.yaml"
  done
  echo "Uploaded the $env renders to Space $space."
}

upload_env dev "$DEV_SPACE" Dev
upload_env staging "$STAGING_SPACE" Staging
upload_env prod "$PROD_SPACE" Prod

echo "Uploaded the Flux control objects to Space $FLEET_SPACE."
echo "Next: ./verify.sh"
