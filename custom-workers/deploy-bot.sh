#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# Run a function bot image in the current Kubernetes cluster, serving a ConfigHub
# Channel as a service account: a Secret holding the service account's private key
# and a Deployment that reads it, applied with kubectl so a demo needs nothing but a
# cluster.
#
# Usage:
#   deploy-bot.sh <space> <bot> <namespace> <image> [KEY=VALUE ...]
#
# <bot> names the service account, the Channel in <space> and the Deployment. Each is
# created if it does not exist. The service account gets Claim on the Channel, and the
# functions the image serves are declared on the Channel from `<image> --print-commands`,
# so the service account needs nothing else. Each KEY=VALUE becomes an environment
# variable of the container, next to CONFIGHUB_URL and CONFIGHUB_CHANNEL.
#
# A key is generated and registered for the service account only when the cluster has no
# Secret for it yet, so running the script again redeploys with the same key.
#
# Environment:
#   CUB                cub binary to invoke (default: cub)
#   DOCKER             docker binary to run the image with --print-commands (default: docker)
#   CONFIGHUB_URL      server the bot connects to (default: the server of the active cub
#                      context; a localhost server is rewritten to host.docker.internal so a
#                      kind node can reach it)
#   KUBECTL_CONTEXT    kubectl context to apply to (default: the current one)
#   IMAGE_PULL_POLICY  default Never, which suits an image loaded into kind

set -euo pipefail

if [[ $# -lt 4 ]]; then
  echo "usage: $0 <space> <bot> <namespace> <image> [KEY=VALUE ...]" >&2
  exit 1
fi

space="$1"
bot="$2"
namespace="$3"
image="$4"
shift 4

cub="${CUB:-cub}"
docker="${DOCKER:-docker}"
kubectl=(kubectl)
if [[ -n "${KUBECTL_CONTEXT:-}" ]]; then
  kubectl+=(--context "$KUBECTL_CONTEXT")
fi

url="${CONFIGHUB_URL:-$($cub context get -o jq=.coordinate.serverURL)}"
url="$(sed -E 's#^(https?://)(localhost|127\.0\.0\.1)#\1host.docker.internal#' <<<"$url")"

$cub serviceaccount create --allow-exists --quiet "$bot"

# Declare the bot's functions on the Channel, and let the service account claim it.
commands="$(mktemp)"
key_dir="$(mktemp -d)"
trap 'rm -rf "$commands" "$key_dir"' EXIT
"$docker" run --rm "$image" --print-commands >"$commands"
if $cub channel get "$space/$bot" >/dev/null 2>&1; then
  $cub channel update --patch --quiet "$space/$bot" \
    --permission "Claim:serviceaccount:$bot" --commands "$commands"
else
  $cub channel create --quiet --space "$space" "$bot" \
    --permission "Claim:serviceaccount:$bot" --commands "$commands"
fi

"${kubectl[@]}" create namespace "$namespace" --dry-run=client -o yaml | "${kubectl[@]}" apply -f -

if ! "${kubectl[@]}" -n "$namespace" get secret "$bot-key" >/dev/null 2>&1; then
  $cub serviceaccount key add "$bot" --generate "$key_dir/$bot.jwk" --description "deploy-bot.sh: $namespace/$bot"
  "${kubectl[@]}" -n "$namespace" create secret generic "$bot-key" \
    --from-file=CONFIGHUB_AUTH_PRIVATE_KEY="$key_dir/$bot.jwk"
fi

env_yaml="        - name: CONFIGHUB_URL
          value: \"$url\"
        - name: CONFIGHUB_CHANNEL
          value: \"$space/$bot\""
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
  name: $bot
  namespace: $namespace
spec:
  replicas: 1
  selector:
    matchLabels:
      app: $bot
  template:
    metadata:
      labels:
        app: $bot
    spec:
      terminationGracePeriodSeconds: 60
      containers:
      - name: bot
        image: $image
        imagePullPolicy: ${IMAGE_PULL_POLICY:-Never}
        env:
$env_yaml
        envFrom:
        - secretRef:
            name: $bot-key
EOF

"${kubectl[@]}" -n "$namespace" rollout status deployment/"$bot" --timeout=120s

for _ in $(seq 1 60); do
  if [[ "$($cub channel get "$space/$bot" -o jq=.Channel.Condition)" == "Claimed" ]]; then
    echo "Bot $namespace/$bot holds the Channel $space/$bot"
    exit 0
  fi
  sleep 1
done
echo "The bot never claimed the Channel $space/$bot; its log:" >&2
"${kubectl[@]}" -n "$namespace" logs deployment/"$bot" --tail=50 >&2 || true
exit 1
