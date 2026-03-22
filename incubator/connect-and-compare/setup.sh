#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
VERIFY=0
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<'EOF_USAGE'
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh
  ./setup.sh --verify
  ./setup.sh --output-dir <path>

This example is fixture-first and does not require a live cluster.
It reads local fixtures and writes local output snapshots only.
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
    --verify)
      VERIFY=1
      shift
      ;;
    --output-dir)
      OUTPUT_DIR="$2"
      shift 2
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
This is a read-only setup plan for connect-and-compare.
No ConfigHub state will be mutated.
No live infrastructure will be mutated.

This example will read local fixtures and generate local evidence snapshots for:
- standalone doctor output
- a compare result for Git intent versus observed bundle state
- a synthetic ChangeSet history view

If you run without --explain, the only writes are local files under:
- ${OUTPUT_DIR}
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  jq -n --arg output_dir "$OUTPUT_DIR" '{
    example: "connect-and-compare",
    mutatesConfighub: false,
    mutatesLiveInfrastructure: false,
    writesLocalFilesOnly: true,
    outputDir: $output_dir,
    requires: ["cub-scout"],
    steps: ["doctor", "connect", "compare", "history"]
  }'
  exit 0
fi

CUB_SCOUT="$(resolve_cub_scout)"
DOCTOR_FIXTURE="$SCRIPT_DIR/testdata/doctor_input.json"
HISTORY_FIXTURE="$SCRIPT_DIR/testdata/history_changesets.json"
GIT_FIXTURE="$SCRIPT_DIR/fixtures/git-repo"
BUNDLE_FIXTURE="$SCRIPT_DIR/fixtures/bundle/dev"
EXPECTED_DIR="$SCRIPT_DIR/expected-output"

mkdir -p "$OUTPUT_DIR"

echo "[1/4] Standalone snapshot: doctor"
CUB_SCOUT_TEST_DOCTOR_INPUT_JSON="$DOCTOR_FIXTURE" "$CUB_SCOUT" doctor --format ascii > "$OUTPUT_DIR/01-doctor.txt"

echo "[2/4] Connect step"
cat > "$OUTPUT_DIR/02-connect.txt" <<'EOF_CONNECT'
$ cub auth login
Connected mode established (fixture replay path for demo).
EOF_CONNECT

echo "[3/4] Compare step: Git intent vs observed bundle"
"$CUB_SCOUT" compare --git-path "$GIT_FIXTURE" --bundle "$BUNDLE_FIXTURE" --json > "$OUTPUT_DIR/03-compare.json"

echo "[4/4] History step: ConfigHub ChangeSet timeline"
CUB_SCOUT_TEST_HISTORY_JSON="$HISTORY_FIXTURE" "$CUB_SCOUT" history deploy/checkout -n prod --since 3650d --format ascii > "$OUTPUT_DIR/04-history.txt"

if [[ "$VERIFY" -eq 1 ]]; then
  diff -u "$EXPECTED_DIR/01-doctor.txt" "$OUTPUT_DIR/01-doctor.txt"
  diff -u "$EXPECTED_DIR/03-compare.json" "$OUTPUT_DIR/03-compare.json"
  diff -u "$EXPECTED_DIR/04-history.txt" "$OUTPUT_DIR/04-history.txt"
  echo "All snapshots match expected output"
fi

echo "Artifacts written to: $OUTPUT_DIR"
