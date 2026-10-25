#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# End-to-end demo of the kube-score example function bot.
#
# This script builds the function bot, runs it locally on a ConfigHub Channel as you,
# and exercises the vet-kube-score function against units with various
# best-practice violations.
#
# Prerequisites:
#   - cub, go installed
#   - kube-score CLI in PATH
#   - Authenticated with: cub auth login
#
# Usage:
#   cd custom-workers/kube-score
#   bash demo.sh

set -euo pipefail

for tool in cub go kube-score; do
  if ! command -v "$tool" &>/dev/null; then
    echo "ERROR: $tool is required but not found in PATH"
    exit 1
  fi
done

# --- Configuration -----------------------------------------------------------

SPACE="kube-score-demo-$(( RANDOM % 9000 + 1000 ))"
CHANNEL="kube-score"
BOT_PID=""

echo "=== Kube-Score Demo ==="
echo "Space: $SPACE"
echo ""

# --- Cleanup trap ------------------------------------------------------------

cleanup() {
  # Kill the function bot's process group: the executable and anything it started.
  if [ -n "$BOT_PID" ] && kill -0 "$BOT_PID" 2>/dev/null; then
    echo "--- Stopping function bot (PID $BOT_PID) ---"
    kill -- -"$BOT_PID" 2>/dev/null || kill "$BOT_PID" 2>/dev/null || true
    wait "$BOT_PID" 2>/dev/null || true
  fi
  echo ""
  echo "Space $SPACE left intact for inspection."
  echo "To clean up: cub space delete --recursive $SPACE"
}
trap cleanup EXIT INT TERM

# --- Build -------------------------------------------------------------------

echo "--- Building kube-score bot ---"
go build -o ./kube-score-bot .
echo ""

# --- Create ConfigHub space --------------------------------------------------

echo "--- Creating ConfigHub space and Channel ---"
cub space create "$SPACE"
cub channel create --space "$SPACE" "$CHANNEL"
echo ""

# --- Start function bot ------------------------------------------------------

BOT_LOG=$(mktemp /tmp/kube-score-bot.XXXXXX)
echo "--- Starting function bot ---"
echo "Bot log: $BOT_LOG"
# The bot authenticates as you, through the active cub context, and declares its
# functions on the Channel when it starts.
set -m  # Enable job control so the background job gets its own process group
./kube-score-bot "$SPACE/$CHANNEL" &>"$BOT_LOG" &
BOT_PID=$!
set +m
echo "Function bot started (PID $BOT_PID)"

echo "--- Waiting for the bot to claim the Channel ---"
for i in $(seq 1 30); do
  if [ "$(cub channel get "$SPACE/$CHANNEL" -o jq=.Channel.Condition)" = "Claimed" ]; then
    break
  fi
  if ! kill -0 "$BOT_PID" 2>/dev/null; then
    echo "FAIL: the function bot exited"
    cat "$BOT_LOG"
    exit 1
  fi
  sleep 1
done
if [ "$(cub channel get "$SPACE/$CHANNEL" -o jq=.Channel.Condition)" != "Claimed" ]; then
  echo "FAIL: the function bot never claimed the Channel"
  cat "$BOT_LOG"
  exit 1
fi
echo "Functions on the Channel:"
cub command list --space "$SPACE"
echo ""

# --- Create test units -------------------------------------------------------

echo "--- Creating unit: good-deploy (well-configured, fewer findings) ---"
cub unit create --space "$SPACE" good-deploy - --toolchain Kubernetes/YAML <<'UNIT'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: good-deploy
  namespace: production
  labels:
    app: good
spec:
  replicas: 3
  selector:
    matchLabels:
      app: good
  template:
    metadata:
      labels:
        app: good
    spec:
      containers:
      - name: web
        image: nginx:1.21
        ports:
        - containerPort: 80
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 200m
            memory: 256Mi
        securityContext:
          readOnlyRootFilesystem: true
          runAsNonRoot: true
          runAsUser: 65534
          runAsGroup: 65534
UNIT
echo ""

echo "--- Creating unit: bad-deploy (missing limits, latest tag, no security context) ---"
cub unit create --space "$SPACE" bad-deploy - --toolchain Kubernetes/YAML <<'UNIT'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: bad-deploy
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: bad
  template:
    metadata:
      labels:
        app: bad
    spec:
      containers:
      - name: web
        image: nginx:latest
UNIT
echo ""

# --- Run validations ---------------------------------------------------------
#
# Note: kube-score flags missing NetworkPolicy, PodDisruptionBudget, and
# ephemeral-storage resources as Critical even on otherwise well-configured
# pods. A single Deployment therefore cannot pass at any threshold without
# accompanying cluster resources. The demo exercises the bot by running
# both deployments at the same Critical threshold — good-deploy still has
# Critical findings, but far fewer, and none for container-level resources,
# security context, or image tag.

echo "=== Running vet-kube-score validations ==="
echo ""

echo "--- vet-kube-score Critical on good-deploy (well-configured; few findings) ---"
cub function do vet-kube-score 'Critical' \
  --space "$SPACE" --where "Slug = 'good-deploy'" --channel "$SPACE/$CHANNEL"
echo ""

echo "--- vet-kube-score Critical on bad-deploy (many findings: resources, security, tag) ---"
cub function do vet-kube-score 'Critical' \
  --space "$SPACE" --where "Slug = 'bad-deploy'" --channel "$SPACE/$CHANNEL"
echo ""

echo "=== Demo complete ==="
