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
CHANNEL="functions"
UNIT="hello-world-demo"
GREETING="Hello from ConfigHub!"
BOT_PID=""
BOT_LOG="$(mktemp)"
OUTPUT_FILE="$(mktemp)"

echo "=== Hello World Function Demo ==="
echo "Space:   $SPACE"
echo "Channel: $SPACE/$CHANNEL"
echo "Unit:    $UNIT"
echo ""

cleanup() {
  if [ -n "$BOT_PID" ] && kill -0 "$BOT_PID" 2>/dev/null; then
    echo "--- Stopping function bot (PID $BOT_PID) ---"
    kill "$BOT_PID" 2>/dev/null || true
    wait "$BOT_PID" 2>/dev/null || true
  fi

  rm -f "$BOT_LOG" "$OUTPUT_FILE"

  if [ "${NOCLEANUP:-}" = "1" ]; then
    echo ""
    echo "=== Skipping cleanup (NOCLEANUP=1) ==="
    echo "Space: $SPACE"
    return
  fi

  echo ""
  echo "=== Cleaning up ==="
  cub space delete "$SPACE" --recursive 2>/dev/null || true
  echo "Done."
}
trap cleanup EXIT INT TERM

echo "--- Building function bot ---"
go build -o ./hello-world-function .
echo ""

echo "--- Creating space and channel ---"
cub space create "$SPACE"
cub channel create --space "$SPACE" "$CHANNEL"
echo ""

echo "--- Starting function bot ---"
# The bot authenticates as you, through the active cub context, and declares its functions
# on the channel when it starts.
./hello-world-function "$SPACE/$CHANNEL" >"$BOT_LOG" 2>&1 &
BOT_PID=$!
for _ in $(seq 1 30); do
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
  echo "FAIL: the function bot never claimed the channel"
  cat "$BOT_LOG"
  exit 1
fi
echo "Function bot started (PID $BOT_PID) and claimed the channel"
echo ""

echo "--- Creating sample unit ---"
cub unit create --space "$SPACE" "$UNIT" test_input.yaml
echo ""

echo "--- Invoking hello-world function ---"
cub function do \
  --space "$SPACE" \
  --channel "$SPACE/$CHANNEL" \
  --where "Slug = '$UNIT'" \
  --show data \
  hello-world "$GREETING" >"$OUTPUT_FILE"
echo ""

echo "--- Verifying annotation ---"
if grep -q "confighub-example/hello-world-greeting: $GREETING" "$OUTPUT_FILE"; then
  echo "PASS: greeting annotation was added"
else
  echo "FAIL: greeting annotation not found in the unit"
  echo ""
  cat "$OUTPUT_FILE"
  echo ""
  cat "$BOT_LOG"
  exit 1
fi
echo ""

echo "=== Demo complete ==="
