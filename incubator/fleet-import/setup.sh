#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example is offline and does not require cluster access.
It reads two existing import JSON files and writes local output only.
EOF_USAGE
}

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

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain)
      EXPLAIN=1
      shift
      ;;
    --explain-json)
      EXPLAIN_JSON=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unexpected argument: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

if [[ "$EXPLAIN" -eq 1 ]]; then
  cat <<EOF_PLAN
This is a read-only setup plan for fleet-import.
No ConfigHub state will be mutated.
No live infrastructure will be mutated.

This example will read two existing import JSON files and generate one unified fleet proposal.

If you run without --explain, the only writes are local files under:
- ${OUTPUT_DIR}
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n --arg output_dir "$OUTPUT_DIR" '{
    example: "fleet-import",
    mutatesConfighub: false,
    mutatesLiveInfrastructure: false,
    writesLocalFilesOnly: true,
    outputDir: $output_dir,
    requires: ["cub-scout"],
    inputs: ["cluster-dev.json", "cluster-prod.json"],
    command: "cub-scout import-cluster-aggregator cluster-dev.json cluster-prod.json --suggest --json"
  }'
  exit 0
fi

mkdir -p "$OUTPUT_DIR"
CUB_SCOUT="$(resolve_cub_scout)"
(
  cd "$SCRIPT_DIR"
  "$CUB_SCOUT" import-cluster-aggregator cluster-dev.json cluster-prod.json --suggest --json > "$OUTPUT_DIR/fleet-summary.json"
)

echo "Saved fleet aggregation output to: $OUTPUT_DIR/fleet-summary.json"
