#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_NAME=${CLUSTER_NAME:-gitops-import-flux}
WITH_WORKER=false
WITH_CONTRAST=false
EXPLAIN=false
EXPLAIN_JSON=false

for arg in "$@"; do
    case "$arg" in
        --with-worker) WITH_WORKER=true ;;
        --with-contrast) WITH_CONTRAST=true ;;
        --explain) EXPLAIN=true ;;
        --explain-json) EXPLAIN_JSON=true ;;
        *)
            echo "Unknown argument: $arg" >&2
            exit 1
            ;;
    esac
done

if $EXPLAIN_JSON; then
    cat <<JSON
{
  "example": "gitops-import-flux",
  "mutates": false,
  "clusterName": "$CLUSTER_NAME",
  "steps": [
    "bin/create-cluster",
    "bin/install-flux",
    "bin/setup-apps"
  ],
  "optionalSteps": [
    "bin/install-worker (requires CUB_SPACE and cub auth)",
    "bin/setup-contrast-apps"
  ],
  "writes": [
    "kind cluster",
    "Flux installation",
    "real Flux podinfo GitRepository and Kustomization",
    "optional D2 brownfield fixtures",
    "optional ConfigHub discovery worker and in-cluster Flux worker with fluxrenderer and fluxoci",
    "local var/<cluster>.kubeconfig and worker pid/log files"
  ]
}
JSON
    exit 0
fi

if $EXPLAIN; then
    cat <<TEXT
Example: gitops-import-flux
Mutates: yes

Default steps:
- create a kind cluster named $CLUSTER_NAME
- install Flux source, helm, and kustomize controllers
- apply a real Flux GitRepository and Kustomization for podinfo

Optional steps:
- install a ConfigHub discovery worker and in-cluster Flux worker with fluxrenderer and fluxoci if --with-worker is set and CUB_SPACE is available
- apply D2 brownfield contrast fixtures from cub-scout if --with-contrast is set

Writes:
- local kind cluster state
- local var/$CLUSTER_NAME.kubeconfig
- local worker pid/log files if --with-worker is used
TEXT
    exit 0
fi

"$SCRIPT_DIR/bin/create-cluster" "$CLUSTER_NAME"
"$SCRIPT_DIR/bin/install-flux" "$CLUSTER_NAME"
"$SCRIPT_DIR/bin/setup-apps" "$CLUSTER_NAME"

if $WITH_CONTRAST; then
    "$SCRIPT_DIR/bin/setup-contrast-apps" "$CLUSTER_NAME"
fi

if $WITH_WORKER; then
    if [ -z "${CUB_SPACE:-}" ]; then
        echo "Error: --with-worker requires CUB_SPACE to be set" >&2
        exit 1
    fi
    "$SCRIPT_DIR/bin/install-worker" "$CLUSTER_NAME"
fi

echo "gitops-import-flux setup complete."
