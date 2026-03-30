#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT
#
# End-to-end demo of the hello-world-bridge example worker.
#
# This script builds the bridge, runs it locally with `cub worker run`,
# and exercises the Apply, Refresh, and Destroy operations.
#
# Prerequisites:
#   - cub installed
#   - Authenticated with: cub auth login
#
# Usage:
#   cd custom-workers/hello-world-bridge
#   bash demo.sh

set -euo pipefail

for tool in cub go; do
  if ! command -v "$tool" &>/dev/null; then
    echo "ERROR: $tool is required but not found in PATH"
    exit 1
  fi
done

# --- Configuration -----------------------------------------------------------

SPACE="hello-bridge-demo-$(( RANDOM % 9000 + 1000 ))"
WORKER="hello-bridge"
BASE_DIR=$(mktemp -d)
TARGET_SUBDIR="dev"
UNIT="myapp"
WORKER_PID=""

echo "=== Hello World Bridge Demo ==="
echo "Space:    $SPACE"
echo "Base dir: $BASE_DIR"
echo ""

# --- Cleanup trap ------------------------------------------------------------

cleanup() {
  # Kill the worker process group (cub worker run + the bridge executable).
  if [ -n "$WORKER_PID" ] && kill -0 "$WORKER_PID" 2>/dev/null; then
    echo "--- Stopping worker (PID $WORKER_PID) ---"
    kill -- -"$WORKER_PID" 2>/dev/null || kill "$WORKER_PID" 2>/dev/null || true
    wait "$WORKER_PID" 2>/dev/null || true
  fi
  if [ "${NOCLEANUP:-}" = "1" ]; then
    echo ""
    echo "=== Skipping cleanup (NOCLEANUP=1) ==="
    echo "Space:    $SPACE"
    echo "Base dir: $BASE_DIR"
    return
  fi
  echo ""
  echo "=== Cleaning up ==="
  cub space delete "$SPACE" --force 2>/dev/null || true
  rm -rf "$BASE_DIR"
  echo "Done."
}
trap cleanup EXIT INT TERM

# --- Build -------------------------------------------------------------------

echo "--- Building hello-world-bridge ---"
go build -o ./hello-world-bridge .
echo ""

# --- Create ConfigHub space --------------------------------------------------

echo "--- Creating ConfigHub space ---"
cub space create "$SPACE"
echo ""

# --- Create target subdirectory ----------------------------------------------

echo "--- Creating target subdirectory: $BASE_DIR/$TARGET_SUBDIR ---"
mkdir -p "$BASE_DIR/$TARGET_SUBDIR"
echo ""

# --- Start worker ------------------------------------------------------------

echo "--- Starting worker with cub worker run ---"
set -m  # Enable job control so the background job gets its own process group
EXAMPLE_BRIDGE_DIR="$BASE_DIR" \
  cub worker run --space "$SPACE" --executable ./hello-world-bridge "$WORKER" &
WORKER_PID=$!
set +m
echo "Worker started (PID $WORKER_PID)"
echo ""

# Wait for the auto-created target to appear
TARGET="${WORKER}-filesystem-kubernetes-yaml-${TARGET_SUBDIR}"
echo "--- Waiting for target $TARGET ---"
cub target get --space "$SPACE" --wait --timeout 60s "$TARGET" &>/dev/null
echo "Target $TARGET is ready."
echo ""

# --- Create unit -------------------------------------------------------------

echo "--- Creating unit: $UNIT ---"
cub unit create --space "$SPACE" "$UNIT" test_input.yaml \
  --target "$TARGET" --toolchain Kubernetes/YAML
echo ""

# --- Apply -------------------------------------------------------------------

echo "--- Applying unit ---"
cub unit apply --space "$SPACE" --wait "$UNIT"
echo ""

EXPECTED_FILE="$BASE_DIR/$TARGET_SUBDIR/$UNIT.yaml"
if [ -f "$EXPECTED_FILE" ]; then
  echo "PASS: File $EXPECTED_FILE was created."
else
  echo "FAIL: File $EXPECTED_FILE was NOT created."
  exit 1
fi
echo ""

# --- Refresh (expect no drift) ----------------------------------------------

echo "--- Refreshing unit (expect no drift) ---"
cub unit refresh --space "$SPACE" --wait "$UNIT"
echo ""

# --- Destroy -----------------------------------------------------------------

echo "--- Destroying unit ---"
cub unit destroy --space "$SPACE" --wait "$UNIT"
echo ""

if [ ! -f "$EXPECTED_FILE" ]; then
  echo "PASS: File $EXPECTED_FILE was removed."
else
  echo "FAIL: File $EXPECTED_FILE still exists after destroy."
  exit 1
fi
echo ""

echo "=== Demo complete ==="
