#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# Run a worker image in the current Kubernetes cluster, connected to ConfigHub
# as a Worker: a Secret holding the Worker's credentials and a Deployment that
# reads it. These are the two objects described in
# https://docs.confighub.com/guide/external-functions/#in-kubernetes, applied
# with kubectl so a demo needs nothing but a cluster.
#
# Usage:
#   deploy-worker.sh <space> <worker> <namespace> <image> [KEY=VALUE ...]
#
# The Worker is created in <space> if it does not exist. Each KEY=VALUE becomes
# an environment variable of the container, next to CONFIGHUB_URL.
#
# Environment:
#   CUB                cub binary to invoke (default: cub)
#   CONFIGHUB_URL      server the worker connects to (default: the server of the
#                      active cub context; a localhost server is rewritten to
#                      host.docker.internal so a kind node can reach it)
#   KUBECTL_CONTEXT    kubectl context to apply to (default: the current one)
#   IMAGE_PULL_POLICY  default Never, which suits an image loaded into kind

set -euo pipefail

if [[ $# -lt 4 ]]; then
  echo "usage: $0 <space> <worker> <namespace> <image> [KEY=VALUE ...]" >&2
  exit 1
fi

space="$1"
worker="$2"
namespace="$3"
image="$4"
shift 4

cub="${CUB:-cub}"
kubectl=(kubectl)
if [[ -n "${KUBECTL_CONTEXT:-}" ]]; then
  kubectl+=(--context "$KUBECTL_CONTEXT")
fi

url="${CONFIGHUB_URL:-$($cub context get -o jq=.coordinate.serverURL)}"
url="$(sed -E 's#^(https?://)(localhost|127\.0\.0\.1)#\1host.docker.internal#' <<<"$url")"

$cub worker create --space "$space" --allow-exists --quiet "$worker"

"${kubectl[@]}" create namespace "$namespace" --dry-run=client -o yaml | "${kubectl[@]}" apply -f -

# The Secret carries exactly what `cub worker get-envs` emits:
# CONFIGHUB_WORKER_ID and CONFIGHUB_WORKER_SECRET.
$cub worker get-envs --no-export --space "$space" "$worker" \
  | "${kubectl[@]}" -n "$namespace" create secret generic "$worker-credentials" \
      --from-env-file=/dev/stdin --dry-run=client -o yaml \
  | "${kubectl[@]}" apply -f -

env_yaml="        - name: CONFIGHUB_URL
          value: \"$url\""
for pair in "$@"; do
  key="${pair%%=*}"
  value="${pair#*=}"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  env_yaml+="
        - name: $key
          value: \"$value\""
done

"${kubectl[@]}" apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $worker
  namespace: $namespace
spec:
  replicas: 1
  selector:
    matchLabels:
      app: $worker
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1
      maxUnavailable: 0
  template:
    metadata:
      labels:
        app: $worker
    spec:
      terminationGracePeriodSeconds: 60
      containers:
      - name: worker
        image: $image
        imagePullPolicy: ${IMAGE_PULL_POLICY:-Never}
        env:
$env_yaml
        envFrom:
        - secretRef:
            name: $worker-credentials
EOF

"${kubectl[@]}" -n "$namespace" rollout status deployment/"$worker" --timeout=120s
