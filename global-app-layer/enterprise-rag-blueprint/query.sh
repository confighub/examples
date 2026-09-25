#!/usr/bin/env bash
set -euo pipefail

# query.sh — Demonstrate the runtime path for STACK=ollama.
#
# Workflow:
#   1. Discover the rag-server pod in the kind cluster (namespace=tenant-acme).
#   2. Port-forward to it.
#   3. Hit /health to dump the env-var configuration ConfigHub fed in.
#   4. POST /answer with the user's question. The rag-server pod calls host
#      Ollama via host.docker.internal:11434 and returns a real Metal-accelerated
#      answer.
#
# Usage:
#   ./query.sh                                  # default question
#   ./query.sh "What is the capital of France?"
#
# Prerequisites:
#   - kind cluster running with the rag-server pod applied via cub
#   - ollama running on the host with llama3.2:3b pulled
#   - kubectl pointing at the kind cluster

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=./lib.sh
source "${SCRIPT_DIR}/lib.sh"

require_jq
load_state

if [[ "${STACK}" != "ollama" ]]; then
  echo "query.sh expects STACK=ollama (currently: ${STACK})." >&2
  echo "Either re-run setup with STACK=ollama, or call rag-server manually." >&2
  exit 1
fi

if ! command -v kubectl >/dev/null 2>&1; then
  echo "Missing kubectl on PATH" >&2
  exit 1
fi

NAMESPACE="${DEPLOY_NAMESPACE}"
QUERY="${1:-What is the capital of France? Answer in one short sentence.}"

if ! curl -fsS http://localhost:11434/api/tags >/dev/null 2>&1; then
  echo "Ollama daemon does not appear to be running on localhost:11434." >&2
  echo "Start it with:  ollama serve   (and pull a model: ollama pull llama3.2:3b)" >&2
  exit 1
fi

if ! kubectl -n "${NAMESPACE}" get deploy/rag-server >/dev/null 2>&1; then
  echo "deployment/rag-server not found in namespace ${NAMESPACE}." >&2
  echo "Run: cub unit apply --space $(deploy_space) $(deployment_unit_name rag-server direct)" >&2
  exit 1
fi

echo "==> Waiting for rag-server pod to be Ready"
kubectl -n "${NAMESPACE}" rollout status deploy/rag-server --timeout=120s

echo "==> Port-forwarding rag-server 8080 -> 18080"
kubectl -n "${NAMESPACE}" port-forward svc/rag-server 18080:8080 >/tmp/rag-pf.log 2>&1 &
PF_PID=$!
trap 'kill "${PF_PID}" 2>/dev/null || true' EXIT
sleep 2

echo "==> /health (env-vars from ConfigHub)"
curl -fsS http://localhost:18080/health | jq

echo
echo "==> /answer  query=${QUERY}"
curl -fsS -X POST http://localhost:18080/answer \
  -H "Content-Type: application/json" \
  -d "$(jq -n --arg q "${QUERY}" '{query: $q}')" \
  | jq

echo
echo "Done."
