#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
DEV_SPACE="${GITOPS_ARGO_BEGINNER_DEV_SPACE:-gitops-argo-beginner-dev}"
PROD_SPACE="${GITOPS_ARGO_BEGINNER_PROD_SPACE:-gitops-argo-beginner-prod}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example renders the apptique app with kustomize for dev and prod, then
uploads each render into its own ConfigHub Space with "cub variant upload".
It does not create or touch a Kubernetes cluster.
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
This is a read-only setup plan for gitops/argo/beginner-applicationset.
Nothing will be mutated.

Conceptual model:

  bootstrap/applicationset.yaml (Argo ApplicationSet, directory generator)
    -> apps/apptique/overlays/dev  (Kustomize overlay, base + dev patch)
    -> apps/apptique/overlays/prod (Kustomize overlay, base + prod patch)

This example will:
- render apps/apptique/overlays/dev with "kustomize build"
- render apps/apptique/overlays/prod with "kustomize build"
- upload the dev render into ConfigHub Space "$DEV_SPACE"
- upload the prod render into ConfigHub Space "$PROD_SPACE"
  using "cub variant upload --component apptique --variant <dev|prod>"

ConfigHub mutations if you run without --explain:
- creates (or updates) Space "$DEV_SPACE" with apptique dev Units
- creates (or updates) Space "$PROD_SPACE" with apptique prod Units

This example never creates or mutates a live Kubernetes cluster or a live
Argo CD installation. The bootstrap ApplicationSet is reference material for
what a real Argo CD instance would apply from this repo.
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg devSpace "$DEV_SPACE" \
    --arg prodSpace "$PROD_SPACE" \
    '{
      example_name: "gitops-argo-beginner-applicationset",
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

require_cmd kustomize
require_cmd cub

mkdir -p "$VAR_DIR"

kustomize build "$SCRIPT_DIR/apps/apptique/overlays/dev" > "$VAR_DIR/rendered-dev.yaml"
kustomize build "$SCRIPT_DIR/apps/apptique/overlays/prod" > "$VAR_DIR/rendered-prod.yaml"

cub variant upload \
  --component apptique --variant dev --environment Dev \
  --space "$DEV_SPACE" \
  "$VAR_DIR/rendered-dev.yaml"

cub variant upload \
  --component apptique --variant prod --environment Prod \
  --space "$PROD_SPACE" \
  "$VAR_DIR/rendered-prod.yaml"

echo "Uploaded apptique dev render to Space $DEV_SPACE."
echo "Uploaded apptique prod render to Space $PROD_SPACE."
echo "Next: ./verify.sh"
