#!/usr/bin/env bash
set -euo pipefail

SPACE="${SPACE:-mini-clean}"
CLUSTER="${CLUSTER:-mini-clean}"
CONTEXT="${CONTEXT:-kind-${CLUSTER}}"
WORKER="${WORKER:-mini-clean-worker}"
TARGET="${TARGET:-mini-clean-target}"
ARGO_URL="${ARGO_URL:-https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git -C "${SCRIPT_DIR}" rev-parse --show-toplevel)"
STATE_DIR="${REPO_ROOT}/var/mini-kubara/case-01"
WORKER_MANIFEST="${STATE_DIR}/${WORKER}.yaml"
APPSET_FILE="${SCRIPT_DIR}/fixtures/applicationsets/mini-clean.yaml"

usage() {
  cat <<EOF
Usage: $0 [--explain]

Prepare Mini-Kubara Case 01 up to the Gate A stop.

Creates or reuses:
  - kind cluster: ${CLUSTER}
  - kubectl context: ${CONTEXT}
  - Argo CD in namespace argocd
  - ConfigHub space: ${SPACE}
  - ConfigHub worker: ${WORKER}
  - ConfigHub target: ${TARGET} -> ${CONTEXT}:argocd

It stops before creating/applying the ApplicationSet unit.

Environment overrides:
  SPACE, CLUSTER, CONTEXT, WORKER, TARGET, ARGO_URL
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

if [[ "${1:-}" == "--explain" ]]; then
  usage
  cat <<EOF

This script performs live setup only. It does not run Gate A or Gate B.

Mutation summary:
  LIVE-WRITE: create/reuse a local kind cluster and install Argo CD.
  CH-WRITE: create/reuse a ConfigHub space, worker, and target.
  LIVE-WRITE: install the ConfigHub worker manifest into the kind cluster.

Why these choices are pinned:
  - Argo CD install uses server-side apply with force-conflicts because the
    ApplicationSet CRD can exceed the client-side last-applied annotation limit.
  - Worker install exports the manifest and applies it with kubectl because a
    prior live run registered the worker in ConfigHub but did not create the
    in-cluster Deployment.
EOF
  exit 0
fi

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "ERROR: missing required tool: $1" >&2
    exit 1
  fi
}

say() {
  printf '\n==> %s\n' "$*"
}

require_tool kind
require_tool kubectl
require_tool cub

say "Checking ConfigHub auth with a read-only space list"
cub space list --json >/dev/null

say "Creating or reusing kind cluster ${CLUSTER}"
if kind get clusters | grep -qx "${CLUSTER}"; then
  echo "kind cluster ${CLUSTER} already exists"
else
  kind create cluster --name "${CLUSTER}" --wait 60s
fi

say "Exporting kubeconfig and selecting ${CONTEXT}"
kind export kubeconfig --name "${CLUSTER}"
kubectl config use-context "${CONTEXT}" >/dev/null
kubectl --context "${CONTEXT}" get nodes

say "Installing Argo CD with server-side apply"
kubectl --context "${CONTEXT}" create namespace argocd --dry-run=client -o yaml \
  | kubectl --context "${CONTEXT}" apply -f -
kubectl --context "${CONTEXT}" -n argocd apply \
  --server-side=true \
  --force-conflicts \
  -f "${ARGO_URL}"

say "Waiting for Argo CD deployments"
kubectl --context "${CONTEXT}" -n argocd wait \
  --for=condition=available deploy --all --timeout=180s

say "Creating or reusing ConfigHub space ${SPACE}"
cub space create "${SPACE}" \
  --allow-exists \
  --label mini-kubara=case-01 \
  --label env=demo

say "Creating or reusing ConfigHub worker ${WORKER}"
cub worker create "${WORKER}" --space "${SPACE}" --allow-exists

say "Exporting and applying ConfigHub worker manifest"
mkdir -p "${STATE_DIR}"
cub worker install "${WORKER}" \
  --space "${SPACE}" \
  -t kubernetes \
  --export \
  --include-secret > "${WORKER_MANIFEST}"
kubectl --context "${CONTEXT}" apply -f "${WORKER_MANIFEST}"
kubectl --context "${CONTEXT}" -n confighub wait \
  --for=condition=available "deploy/${WORKER}" --timeout=120s

say "Creating or reusing ConfigHub target ${TARGET}"
printf -v TARGET_PARAMS '{"KubeContext":"%s","KubeNamespace":"argocd","WaitTimeout":"2m0s"}' "${CONTEXT}"
cub target create "${TARGET}" "${TARGET_PARAMS}" "${WORKER}" \
  --space "${SPACE}" \
  --allow-exists \
  --provider Kubernetes \
  --toolchain Kubernetes/YAML

say "Setup proof"
kubectl --context "${CONTEXT}" -n argocd get deploy
kubectl --context "${CONTEXT}" -n confighub get deploy,pod
cub worker list --space "${SPACE}"
cub target get "${TARGET}" --space "${SPACE}"

if [[ -x "${REPO_ROOT}/scripts/confighub-gui-urls" ]]; then
  say "VIEW IN CONFIGHUB"
  "${REPO_ROOT}/scripts/confighub-gui-urls" --space "${SPACE}" || true
fi

cat <<EOF

CONFIGHUB SAYS: STOP BEFORE GATE A
Setup is ready for Mini-Kubara Case 01.

Next gate needs explicit Y/N before it creates/applies:
  unit:   mini-clean-appset
  file:   ${APPSET_FILE}
  target: ${TARGET}

Gate A should prove the ConfigHub unit receipt, ApplicationSet/mini-clean, one
generated Application/controlplane-mini-clean, and no automated sync policy.
EOF
