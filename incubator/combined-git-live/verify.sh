#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

resolve_cub_scout() {
  if [[ -n "${CUB_SCOUT_BIN:-}" && -x "${CUB_SCOUT_BIN}" ]]; then
    printf '%s\n' "$CUB_SCOUT_BIN"
    return 0
  fi

  if command -v cub-scout >/dev/null 2>&1; then
    command -v cub-scout
    return 0
  fi

  local repo_root="/Users/alexis/Public/github-repos/cub-scout"
  if [[ -x "$repo_root/cub-scout" ]]; then
    printf '%s\n' "$repo_root/cub-scout"
    return 0
  fi

  if [[ -d "$repo_root/cmd/cub-scout" ]]; then
    (cd "$repo_root" && go build -o cub-scout ./cmd/cub-scout >/dev/null)
    printf '%s\n' "$repo_root/cub-scout"
    return 0
  fi

  echo "Could not find cub-scout. Set CUB_SCOUT_BIN or install cub-scout in PATH." >&2
  return 1
}

mkdir -p "$OUTPUT_DIR"
CUB_SCOUT="$(resolve_cub_scout)"
OUT_JSON="$OUTPUT_DIR/alignment.json"
EXPECTED_JSON="$SCRIPT_DIR/expected-output/alignment.json"

"$CUB_SCOUT" combined \
  --git-path "$SCRIPT_DIR/git-repo" \
  --namespace payment-dev,payment-prod \
  --suggest --json > "$OUT_JSON"

actual_summary="$(jq -c '.alignment | group_by(.status) | map({status: .[0].status, count: length}) | sort_by(.status)' "$OUT_JSON")"
expected_summary="$(jq -c '.alignment | group_by(.status) | map({status: .[0].status, count: length}) | sort_by(.status)' "$EXPECTED_JSON")"

if [[ "$actual_summary" != "$expected_summary" ]]; then
  echo "Alignment summary does not match expected output." >&2
  echo "actual:   $actual_summary" >&2
  echo "expected: $expected_summary" >&2
  exit 1
fi

echo "Alignment summary matches expected output."
echo "$actual_summary" | jq

echo "Non-aligned entries:"
jq '.alignment[] | select(.status != "aligned")' "$OUT_JSON"

echo "Saved full output to: $OUT_JSON"
