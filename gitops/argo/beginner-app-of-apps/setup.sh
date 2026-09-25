#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
DEV_SPACE="${GITOPS_ARGO_BEGINNER_AOA_DEV_SPACE:-gitops-argo-beginner-aoa-dev}"
PROD_SPACE="${GITOPS_ARGO_BEGINNER_AOA_PROD_SPACE:-gitops-argo-beginner-aoa-prod}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example collects the plain YAML that each child Argo CD Application
points at, for dev and prod, then uploads each environment into its own
ConfigHub Space with "cub variant upload". It does not create or touch a
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
This is a read-only setup plan for gitops/argo/beginner-app-of-apps.
Nothing will be mutated.

Conceptual model:

  root/root-app.yaml (Argo Application, points at apps/)
    -> apps/apptique-dev.yaml  (child Application -> manifests/apptique/dev,  namespace apptique-dev)
    -> apps/apptique-prod.yaml (child Application -> manifests/apptique/prod, namespace apptique-prod)

This example will:
- collect manifests/apptique/dev/*.yaml into one render
- collect manifests/apptique/prod/*.yaml into one render
- upload the dev render into ConfigHub Space "$DEV_SPACE"
- upload the prod render into ConfigHub Space "$PROD_SPACE"
  using "cub variant upload --component apptique --variant <dev|prod>
  --namespace apptique-<env> --create-namespace", which matches the child
  Applications' destination namespace and their CreateNamespace=true option

ConfigHub mutations if you run without --explain:
- creates (or updates) Space "$DEV_SPACE" with apptique dev Units
- creates (or updates) Space "$PROD_SPACE" with apptique prod Units

This example never creates or mutates a live Kubernetes cluster or a live
Argo CD installation. The root and child Applications are reference material
for what a real Argo CD instance would apply from this repo.
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg devSpace "$DEV_SPACE" \
    --arg prodSpace "$PROD_SPACE" \
    '{
      example_name: "gitops-argo-beginner-app-of-apps",
      mutates: false,
      mutates_confighub: true,
      mutates_live_infra: false,
      spaces: [$devSpace, $prodSpace],
      units: ["apptique-dev", "apptique-prod"],
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

require_cmd cub

mkdir -p "$VAR_DIR"

render_env() {
  local env="$1"
  local out="$VAR_DIR/rendered-${env}.yaml"
  : > "$out"
  for f in "$SCRIPT_DIR/manifests/apptique/${env}"/*.yaml; do
    echo "---" >> "$out"
    cat "$f" >> "$out"
  done
}

render_env dev
render_env prod

cub variant upload \
  --component apptique --variant dev --environment Dev \
  --space "$DEV_SPACE" --namespace apptique-dev --create-namespace \
  "$VAR_DIR/rendered-dev.yaml"

cub variant upload \
  --component apptique --variant prod --environment Prod \
  --space "$PROD_SPACE" --namespace apptique-prod --create-namespace \
  "$VAR_DIR/rendered-prod.yaml"

echo "Uploaded apptique dev render to Space $DEV_SPACE."
echo "Uploaded apptique prod render to Space $PROD_SPACE."
echo "Next: ./verify.sh"
