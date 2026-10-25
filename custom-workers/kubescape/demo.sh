#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# End-to-end demo of the kubescape example function bot.
#
# This script builds the function bot, runs it locally on a ConfigHub Channel as you,
# and exercises the vet-kubescape function against units with various
# security control violations.
#
# Prerequisites:
#   - cub, go installed
#   - kubescape CLI in PATH (or built from source)
#   - Authenticated with: cub auth login
#
# Usage:
#   cd custom-workers/kubescape
#   bash demo.sh

set -euo pipefail

for tool in cub go kubescape; do
  if ! command -v "$tool" &>/dev/null; then
    echo "ERROR: $tool is required but not found in PATH"
    exit 1
  fi
done

# --- Configuration -----------------------------------------------------------

SPACE="kubescape-demo-$(( RANDOM % 9000 + 1000 ))"
CHANNEL="kubescape"
BOT_PID=""

echo "=== Kubescape Demo ==="
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

echo "--- Building kubescape bot ---"
go build -o ./kubescape-bot .
echo ""

# --- Create ConfigHub space --------------------------------------------------

echo "--- Creating ConfigHub space and Channel ---"
cub space create "$SPACE"
cub channel create --space "$SPACE" "$CHANNEL"
echo ""

# --- Start function bot ------------------------------------------------------

BOT_LOG=$(mktemp /tmp/kubescape-bot.XXXXXX)
echo "--- Starting function bot ---"
echo "Bot log: $BOT_LOG"
echo "(kubescape downloads artifacts on first start, this may take a moment)"
# The bot authenticates as you, through the active cub context, and declares its
# functions on the Channel when it starts.
set -m  # Enable job control so the background job gets its own process group
./kubescape-bot "$SPACE/$CHANNEL" &>"$BOT_LOG" &
BOT_PID=$!
set +m
echo "Function bot started (PID $BOT_PID)"

echo "--- Waiting for the bot to claim the Channel ---"
for i in $(seq 1 60); do
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

echo "--- Creating unit: good-deploy (hardened deployment) ---"
cub unit create --space "$SPACE" good-deploy - --toolchain Kubernetes/YAML <<'UNIT'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: good-deploy
  namespace: production
  labels:
    app: good
spec:
  replicas: 1
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
          allowPrivilegeEscalation: false
          capabilities:
            drop:
            - ALL
        readinessProbe:
          httpGet:
            path: /
            port: 80
        livenessProbe:
          httpGet:
            path: /
            port: 80
UNIT
echo ""

echo "--- Creating unit: bad-deploy (privileged, no limits, no probes) ---"
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
        securityContext:
          privileged: true
UNIT
echo ""

# --- Run validations ---------------------------------------------------------

echo "=== Running vet-kubescape validations ==="
echo ""

echo "--- vet-kubescape High on good-deploy (should pass: findings are Medium < High) ---"
cub function do vet-kubescape 'High' \
  --space "$SPACE" --where "Slug = 'good-deploy'" --channel "$SPACE/$CHANNEL"
echo ""

echo "--- vet-kubescape High on bad-deploy (should fail: findings are High >= High) ---"
cub function do vet-kubescape 'High' \
  --space "$SPACE" --where "Slug = 'bad-deploy'" --channel "$SPACE/$CHANNEL"
echo ""

echo "=== Demo complete ==="
