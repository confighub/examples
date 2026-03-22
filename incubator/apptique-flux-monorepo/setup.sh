#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WITH_PROD=0
EXPLAIN=0
EXPLAIN_JSON=0

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "Missing required command: $1" >&2
    exit 1
  }
}

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh [--with-prod]

This example assumes you already have a reachable Kubernetes cluster with Flux installed.
EOF_USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --with-prod)
      WITH_PROD=1
      shift
      ;;
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
This is a read-only setup plan for apptique-flux-monorepo.
Nothing will be mutated.

This example will use the current kubectl context and an existing Flux installation.

It will apply:
- infrastructure/gitrepository.yaml
- clusters/dev/kustomization.yaml
$( [[ "$WITH_PROD" -eq 1 ]] && echo '- clusters/prod/kustomization.yaml' )

Live mutations if you run without --explain:
- Flux GitRepository in flux-system
- Flux Kustomization for apptique-dev
$( [[ "$WITH_PROD" -eq 1 ]] && echo '- Flux Kustomization for apptique-prod' )

Expected live result:
- apptique-dev namespace with frontend deployment
$( [[ "$WITH_PROD" -eq 1 ]] && echo '- apptique-prod namespace with frontend deployment' )
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n \
    --arg example "apptique-flux-monorepo" \
    --argjson withProd "$WITH_PROD" \
    '{
      example: $example,
      mutates: false,
      requires: ["kubectl", "flux"],
      usesCurrentContext: true,
      fluxRequired: true,
      applies: ([
        "infrastructure/gitrepository.yaml",
        "clusters/dev/kustomization.yaml"
      ] + (if $withProd == 1 then ["clusters/prod/kustomization.yaml"] else [] end)),
      expectedNamespaces: (["apptique-dev"] + (if $withProd == 1 then ["apptique-prod"] else [] end))
    }'
  exit 0
fi

require_cmd kubectl
require_cmd flux

kubectl apply -f "$SCRIPT_DIR/infrastructure/gitrepository.yaml"
kubectl apply -f "$SCRIPT_DIR/clusters/dev/kustomization.yaml"
if [[ "$WITH_PROD" -eq 1 ]]; then
  kubectl apply -f "$SCRIPT_DIR/clusters/prod/kustomization.yaml"
fi

echo "Applied apptique Flux monorepo resources."
echo "Next: ./verify.sh$( [[ "$WITH_PROD" -eq 1 ]] && echo ' --with-prod' )"
