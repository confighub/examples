#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GITOPS_DIR="$SCRIPT_DIR/gitops-repo"
VAR_DIR="$SCRIPT_DIR/var"
DEV_SPACE="${GITOPS_ARGO_CI_TO_GITOPS_DEV_SPACE:-gitops-argo-ci-to-gitops-dev}"
PROD_SPACE="${GITOPS_ARGO_CI_TO_GITOPS_PROD_SPACE:-gitops-argo-ci-to-gitops-prod}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example renders the apptique app with kustomize for dev and prod, from
gitops-repo/environments/apptique/, then uploads each render into its own
ConfigHub Space with "cub variant upload". It does not build a container
image, does not push to a registry, does not open a pull request in either
repo, and does not create or touch a Kubernetes cluster.
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
This is a read-only setup plan for gitops/argo/intermediate-ci-to-gitops.
Nothing will be mutated.

Conceptual model:

  app-repo/.github/workflows/build-and-open-gitops-pr.yaml   (illustrative, never run)
    -> opens a pull request against gitops-repo, changing one line:
       gitops-repo/environments/apptique/dev/kustomization.yaml (image tag)

  gitops-repo/.github/workflows/promote-dev-to-prod.yaml      (illustrative, never run)
    -> opens a second pull request, changing one line:
       gitops-repo/environments/apptique/prod/kustomization.yaml (image tag)
       and recording what it promoted in
       gitops-repo/environments/apptique/prod/PROMOTED_FROM.md

  gitops-repo/applications/apptique-dev.yaml  (Argo Application, points at environments/apptique/dev)
  gitops-repo/applications/apptique-prod.yaml (Argo Application, points at environments/apptique/prod)

This example will:
- render gitops-repo/environments/apptique/dev with "kustomize build"
- render gitops-repo/environments/apptique/prod with "kustomize build"
- upload the dev render into ConfigHub Space "$DEV_SPACE"
- upload the prod render into ConfigHub Space "$PROD_SPACE"
  using "cub variant upload --component apptique --variant <dev|prod>
  --namespace apptique-<env>", which matches the Applications' destination
  namespace and their CreateNamespace=true option

ConfigHub mutations if you run without --explain:
- creates (or updates) Space "$DEV_SPACE" with apptique dev Units
- creates (or updates) Space "$PROD_SPACE" with apptique prod Units

This example never builds a container image, never pushes to a registry,
never opens a pull request in either repo, and never creates or mutates a
live Kubernetes cluster or a live Argo CD installation. Both workflow files
are reference material for what each repo's own CI would contain.
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg devSpace "$DEV_SPACE" \
    --arg prodSpace "$PROD_SPACE" \
    '{
      example_name: "gitops-argo-intermediate-ci-to-gitops",
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

kustomize build "$GITOPS_DIR/environments/apptique/dev" > "$VAR_DIR/rendered-dev.yaml"
kustomize build "$GITOPS_DIR/environments/apptique/prod" > "$VAR_DIR/rendered-prod.yaml"

cub variant upload \
  --component apptique --variant dev --environment Dev \
  --space "$DEV_SPACE" --namespace apptique-dev \
  "$VAR_DIR/rendered-dev.yaml"

cub variant upload \
  --component apptique --variant prod --environment Prod \
  --space "$PROD_SPACE" --namespace apptique-prod \
  "$VAR_DIR/rendered-prod.yaml"

echo "Uploaded apptique dev render to Space $DEV_SPACE."
echo "Uploaded apptique prod render to Space $PROD_SPACE."
echo "Next: ./verify.sh"
