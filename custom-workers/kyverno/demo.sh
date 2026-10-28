#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# End-to-end demo of the kyverno example function bot.
#
# Unlike the admission-webhook example, this bot uses the kyverno CLI for
# offline validation — policies are passed as a function parameter, so Kyverno
# does not need to be deployed in the cluster.
#
# Prerequisites:
#   - kind, kubectl, cub, docker installed
#   - Authenticated with: cub auth login
#
# Usage:
#   cd custom-workers/kyverno
#   bash demo.sh

set -euo pipefail

# --- Prerequisites -----------------------------------------------------------

for tool in kind kubectl cub docker; do
  if ! command -v "$tool" &>/dev/null; then
    echo "ERROR: $tool is required but not found in PATH"
    exit 1
  fi
done

# --- Configuration -----------------------------------------------------------

CLUSTER_NAME="kyverno-cli-demo-$(( RANDOM % 9000 + 1000 ))"
SUFFIX="$(( RANDOM % 9000 + 1000 ))"
SPACE="kyverno-cli-demo-$SUFFIX"
# The bot's service account, its Channel and its Deployment share this name. Service
# accounts belong to the organization, so the name carries the run's suffix.
BOT="kyverno-cli-bot-$SUFFIX"
BOT_NAMESPACE="kyverno-cli-bot"
IMAGE_NAME="kyverno-cli-bot:demo"

echo "=== Kyverno CLI Demo ==="
echo "Cluster:  $CLUSTER_NAME"
echo "Space:    $SPACE"
echo ""

# --- Cleanup trap ------------------------------------------------------------

cleanup() {
  if [ "${NOCLEANUP:-}" = "1" ]; then
    echo ""
    echo "=== Skipping cleanup (NOCLEANUP=1) ==="
    echo "Cluster: $CLUSTER_NAME"
    echo "Space:   $SPACE"
    echo "Service account: $BOT"
    return
  fi
  echo ""
  echo "=== Cleaning up ==="
  kind delete cluster --name "$CLUSTER_NAME" 2>/dev/null || true
  cub space delete "$SPACE" --recursive 2>/dev/null || true
  cub serviceaccount delete "$BOT" 2>/dev/null || true
  echo "Done."
}
trap cleanup EXIT

# --- Create kind cluster -----------------------------------------------------

echo "--- Creating kind cluster ---"
kind create cluster --name "$CLUSTER_NAME"
kubectl cluster-info --context "kind-$CLUSTER_NAME"
echo ""

# --- Build and load Docker image ---------------------------------------------

echo "--- Building Docker image ---"
docker build -t "$IMAGE_NAME" .

echo "--- Loading image into kind ---"
kind load docker-image "$IMAGE_NAME" --name "$CLUSTER_NAME"
echo ""

# --- Create ConfigHub space --------------------------------------------------

echo "--- Creating ConfigHub space ---"
cub space create "$SPACE"
echo ""

# --- Deploy the kyverno CLI function bot -------------------------------------

echo "--- Deploying kyverno CLI function bot ---"
# A service account with Claim on a Channel that declares the bot's functions, a Secret
# with the service account's key, and a Deployment that reads it.
"$(dirname "$0")/../deploy-bot.sh" "$SPACE" "$BOT" "$BOT_NAMESPACE" "$IMAGE_NAME"
echo "Kyverno CLI function bot is ready."
echo ""

# --- Load policies and resources from demo-data/ -----------------------------

DATA_DIR="$(dirname "$0")/demo-data"

REQUIRE_LABELS_POLICY=$(<"$DATA_DIR/require-labels-policy.yaml")
DISALLOW_LATEST_TAG_VAP=$(<"$DATA_DIR/disallow-latest-tag-vap.yaml")
MIXED_POLICIES="${REQUIRE_LABELS_POLICY}
---
${DISALLOW_LATEST_TAG_VAP}"

# --- Create test units -------------------------------------------------------

echo "--- Creating test unit: deploy-with-labels (should pass require-labels) ---"
cub unit create --space "$SPACE" deploy-with-labels - --toolchain Kubernetes/YAML \
  < "$DATA_DIR/deploy-with-labels.yaml"
echo ""

echo "--- Creating test unit: deploy-without-labels (should fail require-labels) ---"
cub unit create --space "$SPACE" deploy-without-labels - --toolchain Kubernetes/YAML \
  < "$DATA_DIR/deploy-without-labels.yaml"
echo ""

echo "--- Creating test unit: deploy-latest-tag (should fail disallow-latest-tag VAP) ---"
cub unit create --space "$SPACE" deploy-latest-tag - --toolchain Kubernetes/YAML \
  < "$DATA_DIR/deploy-latest-tag.yaml"
echo ""

# --- Run validation with Kyverno ValidatingPolicy ----------------------------

echo "=== Testing with Kyverno ValidatingPolicy ==="
echo ""

echo "--- Running vet-kyverno on deploy-with-labels (should pass) ---"
cub function do vet-kyverno "$REQUIRE_LABELS_POLICY" \
  --space "$SPACE" --unit deploy-with-labels --channel "$SPACE/$BOT"
echo ""

echo "--- Running vet-kyverno on deploy-without-labels (should fail) ---"
cub function do vet-kyverno "$REQUIRE_LABELS_POLICY" \
  --space "$SPACE" --unit deploy-without-labels --channel "$SPACE/$BOT"
echo ""

# --- Run validation with Kubernetes ValidatingAdmissionPolicy -----------------

echo "=== Testing with Kubernetes ValidatingAdmissionPolicy ==="
echo ""

echo "--- Running vet-kyverno on deploy-with-labels (should pass) ---"
cub function do vet-kyverno "$DISALLOW_LATEST_TAG_VAP" \
  --space "$SPACE" --unit deploy-with-labels --channel "$SPACE/$BOT"
echo ""

echo "--- Running vet-kyverno on deploy-latest-tag (should fail) ---"
cub function do vet-kyverno "$DISALLOW_LATEST_TAG_VAP" \
  --space "$SPACE" --unit deploy-latest-tag --channel "$SPACE/$BOT"
echo ""

# --- Run validation with mixed policies --------------------------------------

echo "=== Testing with mixed ValidatingPolicy + ValidatingAdmissionPolicy ==="
echo ""

echo "--- Running vet-kyverno on deploy-latest-tag (should fail both) ---"
cub function do vet-kyverno "$MIXED_POLICIES" \
  --space "$SPACE" --unit deploy-latest-tag --channel "$SPACE/$BOT"
echo ""

echo "=== Demo complete ==="
