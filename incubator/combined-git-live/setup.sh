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

This example assumes you already have a reachable Kubernetes cluster.
It applies local fixture workloads that simulate a mixed Flux plus native cluster state.
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
This is a read-only setup plan for combined-git-live.
No ConfigHub state will be mutated.

This example will use the current kubectl context and apply local cluster fixtures that simulate:
- four Flux-managed Deployments across dev and prod
- one native cluster-only Deployment

Live mutations if you run without --explain:
- Namespace/payment-dev
- Namespace/payment-prod
- the fixture Deployments in those namespaces
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n '{
    example: "combined-git-live",
    mutatesConfighub: false,
    mutatesLiveInfrastructure: true,
    requires: ["kubectl"],
    applies: ["cluster-fixtures/deployments.yaml"],
    expectedNamespaces: ["payment-dev", "payment-prod"],
    expectedDeployments: ["payment-api", "payment-worker", "cache-warmer"]
  }'
  exit 0
fi

require_cmd kubectl
kubectl create namespace payment-dev --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace payment-prod --dry-run=client -o yaml | kubectl apply -f -
kubectl apply -f "$SCRIPT_DIR/cluster-fixtures/deployments.yaml"

echo "Applied combined-git-live cluster fixtures."
echo "Next: ./verify.sh"
