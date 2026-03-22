#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example assumes you already have a reachable Kubernetes cluster with Argo CD installed.
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
This is a read-only setup plan for apptique-argo-app-of-apps.
Nothing will be mutated.

This example will use the current kubectl context and an existing Argo CD installation.

It will apply:
- root/root-app.yaml

Live mutations if you run without --explain:
- one root Argo Application in argocd
- generated child Applications for dev and prod
- the resulting namespaces, deployment, and service
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n '{
    example: "apptique-argo-app-of-apps",
    mutates: false,
    requires: ["kubectl"],
    usesCurrentContext: true,
    argoRequired: true,
    applies: ["root/root-app.yaml"],
    expectedApplications: ["apptique-apps", "apptique-dev", "apptique-prod"],
    expectedNamespaces: ["apptique-dev", "apptique-prod"]
  }'
  exit 0
fi

require_cmd kubectl
kubectl apply -f "$SCRIPT_DIR/root/root-app.yaml"

echo "Applied apptique Argo app-of-apps resources."
echo "Next: ./verify.sh"
