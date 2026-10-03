#!/usr/bin/env bash
# Copyright (C) ConfigHub, Inc.
# SPDX-License-Identifier: MIT

set -euo pipefail

for tool in cub go grep; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "ERROR: $tool is required but was not found in PATH"
    exit 1
  fi
done

SPACE="hello-world-function-demo-$(( RANDOM % 9000 + 1000 ))"
WORKER="hello-world-worker"
UNIT="hello-world-demo"
GREETING="Hello from ConfigHub!"
WORKER_PID=""
OUTPUT_FILE="$(mktemp)"

echo "=== Hello World Function Demo ==="
echo "Space:  $SPACE"
echo "Worker: $WORKER"
echo "Unit:   $UNIT"
echo ""

cleanup() {
  if [ -n "$WORKER_PID" ] && kill -0 "$WORKER_PID" 2>/dev/null; then
    echo "--- Stopping worker (PID $WORKER_PID) ---"
    kill -- -"$WORKER_PID" 2>/dev/null || kill "$WORKER_PID" 2>/dev/null || true
    wait "$WORKER_PID" 2>/dev/null || true
  fi

  rm -f "$OUTPUT_FILE"

  if [ "${NOCLEANUP:-}" = "1" ]; then
    echo ""
    echo "=== Skipping cleanup (NOCLEANUP=1) ==="
    echo "Space: $SPACE"
    return
  fi

  echo ""
  echo "=== Cleaning up ==="
  cub space delete "$SPACE" --force 2>/dev/null || true
  echo "Done."
}
trap cleanup EXIT INT TERM

echo "--- Building worker ---"
go build -o ./hello-world-function .
echo ""

echo "--- Creating space ---"
cub space create "$SPACE"
echo ""

echo "--- Starting function executor ---"
set -m
cub worker create --space "$SPACE" "$WORKER"
(
  # The function executor reads the Worker's credentials and the server from its environment.
  eval "$(cub worker get-envs --space "$SPACE" "$WORKER")"
  export CONFIGHUB_URL="${CONFIGHUB_URL:-$(cub context get -o jq=.coordinate.serverURL)}"
  exec ./hello-world-function
) &
WORKER_PID=$!
set +m
echo "Function executor started (PID $WORKER_PID)"
echo ""

echo "--- Creating sample unit ---"
cub unit create --space "$SPACE" "$UNIT" test_input.yaml
echo ""

echo "--- Invoking hello-world function ---"
success=0
for attempt in 1 2 3 4 5; do
  if cub function do \
    --space "$SPACE" \
    --worker "$WORKER" \
    --where "Slug = '$UNIT'" \
    --show data \
    hello-world "$GREETING" >"$OUTPUT_FILE"; then
    success=1
    break
  fi
  echo "Worker not ready yet; retrying ($attempt/5)..."
  sleep 2
done

if [ "$success" -ne 1 ]; then
  echo "FAIL: function invocation never succeeded"
  exit 1
fi

echo "--- Verifying annotation ---"
if grep -q "confighub-example/hello-world-greeting: $GREETING" "$OUTPUT_FILE"; then
  echo "PASS: greeting annotation was added"
else
  echo "FAIL: greeting annotation not found in function output"
  echo ""
  cat "$OUTPUT_FILE"
  exit 1
fi
echo ""

echo "=== Demo complete ==="
