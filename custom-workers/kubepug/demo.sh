#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# End-to-end demo of the kubepug example function bot.
#
# This script builds the function bot, runs it locally on a ConfigHub Channel as you,
# and exercises the vet-kubepug function against units with deprecated and
# deleted Kubernetes APIs.
#
# Prerequisites:
#   - cub, go installed
#   - kubepug CLI in PATH (or built from source)
#   - Authenticated with: cub auth login
#
# Usage:
#   cd custom-workers/kubepug
#   bash demo.sh

set -euo pipefail

for tool in cub go kubepug; do
  if ! command -v "$tool" &>/dev/null; then
    echo "ERROR: $tool is required but not found in PATH"
    exit 1
  fi
done

# --- Configuration -----------------------------------------------------------

SPACE="kubepug-demo-$(( RANDOM % 9000 + 1000 ))"
CHANNEL="kubepug"
BOT_PID=""

echo "=== Kubepug Demo ==="
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

echo "--- Building kubepug bot ---"
go build -o ./kubepug-bot .
echo ""

# --- Create ConfigHub space --------------------------------------------------

echo "--- Creating ConfigHub space and Channel ---"
cub space create "$SPACE"
cub channel create --space "$SPACE" "$CHANNEL"
echo ""

# --- Start function bot ------------------------------------------------------

BOT_LOG=$(mktemp /tmp/kubepug-bot.XXXXXX)
echo "--- Starting function bot ---"
echo "Bot log: $BOT_LOG"
# The bot authenticates as you, through the active cub context, and declares its
# functions on the Channel when it starts.
set -m  # Enable job control so the background job gets its own process group
./kubepug-bot "$SPACE/$CHANNEL" &>"$BOT_LOG" &
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

echo "--- Creating unit: good-deploy (apps/v1 Deployment, should pass) ---"
cub unit create --space "$SPACE" good-deploy - --toolchain Kubernetes/YAML <<'UNIT'
apiVersion: apps/v1
kind: Deployment
metadata:
  name: good-deploy
  namespace: default
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
UNIT
echo ""

echo "--- Creating unit: old-cronjob (batch/v1beta1 CronJob, deprecated in 1.21, deleted in 1.25) ---"
cub unit create --space "$SPACE" old-cronjob - --toolchain Kubernetes/YAML <<'UNIT'
apiVersion: batch/v1beta1
kind: CronJob
metadata:
  name: old-cronjob
  namespace: default
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: busybox
            command: ["echo", "hello"]
          restartPolicy: OnFailure
UNIT
echo ""

echo "--- Creating unit: old-ingress (extensions/v1beta1 Ingress, deleted in 1.22) ---"
cub unit create --space "$SPACE" old-ingress - --toolchain Kubernetes/YAML <<'UNIT'
apiVersion: extensions/v1beta1
kind: Ingress
metadata:
  name: old-ingress
  namespace: default
spec:
  rules:
  - host: example.com
    http:
      paths:
      - path: /
        backend:
          serviceName: web
          servicePort: 80
UNIT
echo ""

# --- Run validations ---------------------------------------------------------

echo "=== Running vet-kubepug validations ==="
echo ""

echo "--- vet-kubepug v1.25 Low on good-deploy (should pass: no findings) ---"
cub function do vet-kubepug 'v1.25' 'Low' \
  --space "$SPACE" --where "Slug = 'good-deploy'" --channel "$SPACE/$CHANNEL"
echo ""

echo "--- vet-kubepug v1.21 Critical on old-cronjob (should pass: deprecated=High < Critical) ---"
cub function do vet-kubepug 'v1.21' 'Critical' \
  --space "$SPACE" --where "Slug = 'old-cronjob'" --channel "$SPACE/$CHANNEL"
echo ""

echo "--- vet-kubepug v1.21 High on old-cronjob (should fail: deprecated=High >= High) ---"
cub function do vet-kubepug 'v1.21' 'High' \
  --space "$SPACE" --where "Slug = 'old-cronjob'" --channel "$SPACE/$CHANNEL"
echo ""

echo "--- vet-kubepug v1.25 Critical on old-cronjob (should fail: deleted=Critical >= Critical) ---"
cub function do vet-kubepug 'v1.25' 'Critical' \
  --space "$SPACE" --where "Slug = 'old-cronjob'" --channel "$SPACE/$CHANNEL"
echo ""

echo "--- vet-kubepug v1.22 Critical on old-ingress (should fail: deleted=Critical >= Critical) ---"
cub function do vet-kubepug 'v1.22' 'Critical' \
  --space "$SPACE" --where "Slug = 'old-ingress'" --channel "$SPACE/$CHANNEL"
echo ""

echo "=== Demo complete ==="
