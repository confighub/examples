#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VAR_DIR="$SCRIPT_DIR/var"
ARGO_CONTROL_SPACE="${GITOPS_EXPERT_BROKEN_STATES_ARGO_CONTROL_SPACE:-gitops-expert-broken-states-argo-control}"
FLUX_CONTROL_SPACE="${GITOPS_EXPERT_BROKEN_STATES_FLUX_CONTROL_SPACE:-gitops-expert-broken-states-flux-control}"
WORKLOAD_SPACE="${GITOPS_EXPERT_BROKEN_STATES_SPACE:-gitops-expert-broken-states}"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example renders the one healthy app this example uses (apptique) with
kustomize, plus the Argo CD Application and Flux Kustomization/GitRepository
that would deliver it, then uploads only the healthy renders into ConfigHub
with "cub variant upload". It never creates or touches a Kubernetes cluster,
never installs or talks to Argo CD or Flux, and never uploads any of the
three broken overlays under apps/apptique/overlays/ (failed-sync,
bad-commit) or runs any command in scenarios/*/break.sh.
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
This is a read-only setup plan for gitops/expert-broken-states.
Nothing will be mutated.

Conceptual model:

  apps/apptique/overlays/healthy/     (the only state this example uploads)
  apps/apptique/overlays/failed-sync/ (local only: a missing-CRD resource added)
  apps/apptique/overlays/bad-commit/  (local only: Service targetPort moved, container left alone)

  argo/application.yaml   an Argo CD Application pointed at overlays/healthy
  flux/apps.yaml          a Flux Kustomization pointed at the same path
  flux/gitrepository.yaml the GitRepository that Kustomization reads from

A ConfigHub Space belongs to one Component: uploading a second Component
into a Space re-links that Space to the new Component and overwrites its
labels. So this example uses one Space per Component, three Spaces in all,
not one shared control Space.

This example will:
- render apps/apptique/overlays/healthy with "kustomize build"
- render argo/ and flux/ (the control objects) with "kustomize build", each
  on its own, since they carry different namespaces (argocd, flux-system)
- upload the Argo control render into ConfigHub Space "$ARGO_CONTROL_SPACE"
  using "cub variant upload --component argo-control --namespace argocd"
- upload the Flux control render into ConfigHub Space "$FLUX_CONTROL_SPACE"
  using "cub variant upload --component flux-control --namespace flux-system"
- upload the healthy app render into ConfigHub Space "$WORKLOAD_SPACE" using
  "cub variant upload --component apptique --variant healthy --namespace apptique-broken-states"

ConfigHub mutations if you run without --explain:
- creates (or updates) Space "$ARGO_CONTROL_SPACE" with the Argo Application
- creates (or updates) Space "$FLUX_CONTROL_SPACE" with the Flux
  Kustomization and GitRepository
- creates (or updates) Space "$WORKLOAD_SPACE" with the healthy apptique Unit

This example never creates or mutates a live Kubernetes cluster, never
installs or talks to Argo CD or Flux, and never uploads the failed-sync or
bad-commit overlays. Those exist only so you can render and inspect them
locally; see scenarios/*/README.md for what each one does on a live
cluster, and scenarios/*/break.sh for the exact commands a person would run
in their own terminal to see it (never run by this script).
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg argoControlSpace "$ARGO_CONTROL_SPACE" \
    --arg fluxControlSpace "$FLUX_CONTROL_SPACE" \
    --arg workloadSpace "$WORKLOAD_SPACE" \
    '{
      example_name: "gitops-expert-broken-states",
      mutates: false,
      mutates_confighub: true,
      mutates_live_infra: false,
      spaces: [$argoControlSpace, $fluxControlSpace, $workloadSpace],
      units: ["argo-control", "flux-control", "frontend"],
      apps: ["apptique"],
      scenarios: ["drift", "failed-sync", "bad-commit"],
      uploaded_states: ["healthy"],
      not_uploaded_states: ["failed-sync", "bad-commit"],
      namespaces: ["argocd", "flux-system", "apptique-broken-states"],
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

kustomize build "$SCRIPT_DIR/argo" > "$VAR_DIR/rendered-argo-control.yaml"
kustomize build "$SCRIPT_DIR/flux" > "$VAR_DIR/rendered-flux-control.yaml"
kustomize build "$SCRIPT_DIR/apps/apptique/overlays/healthy" > "$VAR_DIR/rendered-healthy.yaml"

cub variant upload \
  --component argo-control --variant control --environment Control \
  --namespace argocd \
  --space "$ARGO_CONTROL_SPACE" \
  "$VAR_DIR/rendered-argo-control.yaml"

cub variant upload \
  --component flux-control --variant control --environment Control \
  --namespace flux-system \
  --space "$FLUX_CONTROL_SPACE" \
  "$VAR_DIR/rendered-flux-control.yaml"

cub variant upload \
  --component apptique --variant healthy --environment Healthy \
  --namespace apptique-broken-states \
  --space "$WORKLOAD_SPACE" \
  "$VAR_DIR/rendered-healthy.yaml"

echo "Uploaded the Argo control objects to Space $ARGO_CONTROL_SPACE (component argo-control)."
echo "Uploaded the Flux control objects to Space $FLUX_CONTROL_SPACE (component flux-control)."
echo "Uploaded the healthy apptique render to Space $WORKLOAD_SPACE."
echo "The failed-sync and bad-commit overlays were not uploaded."
echo "Next: ./verify.sh"
