#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUNDLE_DIR="$SCRIPT_DIR/fixtures/debug-bundle"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
EXPLAIN=0
EXPLAIN_JSON=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain) EXPLAIN=1 ;;
    --explain-json) EXPLAIN_JSON=1 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
  shift
done

if [[ $EXPLAIN -eq 1 ]]; then
  cat <<TEXT
This example will:
- read fixtures/debug-bundle as an offline debug bundle
- run cub-scout bundle inspect --format json
- run cub-scout bundle replay --format json
- run cub-scout bundle summarize --format json
- write JSON output under sample-output/
- not write ConfigHub state
- not mutate live infrastructure
TEXT
  exit 0
fi

if [[ $EXPLAIN_JSON -eq 1 ]]; then
  jq -n '{example:"artifact-workflow", mutatesConfighub:false, mutatesLiveInfrastructure:false, source:"fixtures/debug-bundle"}'
  exit 0
fi

command -v cub-scout >/dev/null
mkdir -p "$OUTPUT_DIR"

cub-scout bundle inspect "$BUNDLE_DIR" --format json > "$OUTPUT_DIR/bundle-inspect.json"
cub-scout bundle replay "$BUNDLE_DIR" --format json > "$OUTPUT_DIR/bundle-replay-drift.json"
cub-scout bundle summarize "$BUNDLE_DIR" --format json > "$OUTPUT_DIR/bundle-summarize.json"

echo "Saved bundle outputs to: $OUTPUT_DIR"
