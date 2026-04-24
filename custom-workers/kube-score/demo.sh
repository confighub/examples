#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# End-to-end demo of the kube-score example worker.
#
# This script builds the worker, runs it locally with `cub worker run`,
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
WORKER="kube-score-worker"
WORKER_PID=""

echo "=== Kube-Score Demo ==="
echo "Space: $SPACE"
echo ""

# --- Cleanup trap ------------------------------------------------------------

cleanup() {
  # Kill the worker process group (cub worker run + the worker executable).
  if [ -n "$WORKER_PID" ] && kill -0 "$WORKER_PID" 2>/dev/null; then
    echo "--- Stopping worker (PID $WORKER_PID) ---"
    kill -- -"$WORKER_PID" 2>/dev/null || kill "$WORKER_PID" 2>/dev/null || true
    wait "$WORKER_PID" 2>/dev/null || true
  fi
  echo ""
  echo "Space $SPACE left intact for inspection."
  echo "To clean up: cub space delete --recursive $SPACE"
}
trap cleanup EXIT INT TERM

# --- Build -------------------------------------------------------------------

echo "--- Building kube-score worker ---"
go build -o ./kube-score-worker .
echo ""

# --- Create ConfigHub space --------------------------------------------------

echo "--- Creating ConfigHub space ---"
cub space create "$SPACE"
echo ""

# --- Start worker ------------------------------------------------------------

WORKER_LOG=$(mktemp /tmp/kube-score-worker.XXXXXX)
echo "--- Starting worker with cub worker run ---"
echo "Worker log: $WORKER_LOG"
set -m  # Enable job control so the background job gets its own process group
cub worker run --space "$SPACE" --executable ./kube-score-worker "$WORKER" &>"$WORKER_LOG" &
WORKER_PID=$!
set +m
echo "Worker started (PID $WORKER_PID)"

echo "--- Waiting for worker to be ready ---"
for i in $(seq 1 30); do
  if cub worker list-function --space "$SPACE" "$WORKER" --names 2>/dev/null | grep -q vet-kube-score; then
    break
  fi
  sleep 1
done
echo "Worker functions:"
cub worker list-function --space "$SPACE" "$WORKER"
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
# accompanying cluster resources. The demo exercises the worker by running
# both deployments at the same Critical threshold — good-deploy still has
# Critical findings, but far fewer, and none for container-level resources,
# security context, or image tag.

echo "=== Running vet-kube-score validations ==="
echo ""

echo "--- vet-kube-score Critical on good-deploy (well-configured; few findings) ---"
cub function do vet-kube-score 'Critical' \
  --space "$SPACE" --where "Slug = 'good-deploy'" --worker "$SPACE/$WORKER"
echo ""

echo "--- vet-kube-score Critical on bad-deploy (many findings: resources, security, tag) ---"
cub function do vet-kube-score 'Critical' \
  --space "$SPACE" --where "Slug = 'bad-deploy'" --worker "$SPACE/$WORKER"
echo ""

echo "=== Demo complete ==="
