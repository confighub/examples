#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURE_DIR="$SCRIPT_DIR/fixtures/summary-store"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
SUMMARY_WINDOW="${SUMMARY_WINDOW:-87600h}"
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
- read fixtures/summary-store as CUB_SCOUT_SUMMARY_DIR
- run cub-scout summary list --since ${SUMMARY_WINDOW} --json
- run cub-scout summary list --since ${SUMMARY_WINDOW} --cluster kind-dev --namespace prod --json
- run cub-scout summary slack --dry-run --since ${SUMMARY_WINDOW} --cluster kind-dev
- write JSON output under sample-output/
- not write ConfigHub state
- not mutate live infrastructure
TEXT
  exit 0
fi

if [[ $EXPLAIN_JSON -eq 1 ]]; then
  jq -n --arg summaryWindow "$SUMMARY_WINDOW" '{example:"connected-summary-storage", mutatesConfighub:false, mutatesLiveInfrastructure:false, source:"fixtures/summary-store", summaryWindow:$summaryWindow}'
  exit 0
fi

command -v cub-scout >/dev/null
mkdir -p "$OUTPUT_DIR"
export CUB_SCOUT_SUMMARY_DIR="$FIXTURE_DIR"

cub-scout summary list --since "$SUMMARY_WINDOW" --json > "$OUTPUT_DIR/summary-list-all.json"
cub-scout summary list --since "$SUMMARY_WINDOW" --cluster kind-dev --namespace prod --json > "$OUTPUT_DIR/summary-list-kind-dev-prod.json"
cub-scout summary slack --dry-run --since "$SUMMARY_WINDOW" --cluster kind-dev > "$OUTPUT_DIR/summary-slack-dry-run.json"

echo "Saved summary outputs to: $OUTPUT_DIR"
