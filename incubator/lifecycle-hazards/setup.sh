#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
FIXTURE="$SCRIPT_DIR/fixtures/helm-hooks.yaml"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example is file-based and does not require cluster access.
It reads a copied manifest file and writes local JSON output only.
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
This is a read-only setup plan for lifecycle-hazards.
No ConfigHub state will be mutated.
No live infrastructure will be mutated.

If you run without --explain, this example will:
- read ${FIXTURE}
- run cub-scout map hooks --file --format json
- run cub-scout scan --file --lifecycle-hazards --json
- write local output under ${OUTPUT_DIR}
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n --arg output_dir "$OUTPUT_DIR" --arg fixture "$FIXTURE" '{
    example: "lifecycle-hazards",
    mutatesConfighub: false,
    mutatesLiveInfrastructure: false,
    writesLocalFilesOnly: true,
    outputDir: $output_dir,
    fixture: $fixture,
    requires: ["cub-scout", "jq"],
    commands: [
      "cub-scout map hooks --file ... --format json",
      "cub-scout scan --file ... --lifecycle-hazards --json"
    ]
  }'
  exit 0
fi

command -v jq >/dev/null 2>&1 || { echo "Missing required tool: jq" >&2; exit 1; }
CUB_SCOUT="$(resolve_cub_scout)"
mkdir -p "$OUTPUT_DIR"
rm -f "$OUTPUT_DIR"/*.json

"$CUB_SCOUT" map hooks --file "$FIXTURE" --format json > "$OUTPUT_DIR/hooks.json"
"$CUB_SCOUT" scan --file "$FIXTURE" --lifecycle-hazards --json > "$OUTPUT_DIR/lifecycle-scan.json" 2>/dev/null || true

jq -S . "$OUTPUT_DIR/hooks.json" > "$OUTPUT_DIR/hooks.normalized.json"
jq '(.lifecycleHazards.scannedAt) = "TIMESTAMP" | (.static.scannedAt) = "TIMESTAMP" | (.static.file) = "FIXTURE_PATH"' "$OUTPUT_DIR/lifecycle-scan.json" | jq -S . > "$OUTPUT_DIR/lifecycle-scan.normalized.json"

echo "Saved hook inventory to: $OUTPUT_DIR/hooks.json"
echo "Saved lifecycle hazard scan to: $OUTPUT_DIR/lifecycle-scan.json"
