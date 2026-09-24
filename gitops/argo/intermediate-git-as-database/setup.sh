#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_DIR="$SCRIPT_DIR/repo"
SCOUT="$SCRIPT_DIR/../../../prompts/scout/scout.py"
EXPLAIN=0
EXPLAIN_JSON=0

usage() {
  cat <<EOF_USAGE
Usage:
  ./setup.sh --explain
  ./setup.sh --explain-json
  ./setup.sh

This example runs scout (prompts/scout/scout.py) over repo/, a small Argo CD
fleet repo with one planted instance of each of seven problems. It is
read-only: it does not call ConfigHub, touch a cluster, or write any file.
EOF_USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain) EXPLAIN=1; shift ;;
    --explain-json) EXPLAIN_JSON=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unexpected argument: $1" >&2; usage >&2; exit 1 ;;
  esac
done

if [[ "$EXPLAIN" -eq 1 ]]; then
  cat <<EOF_PLAN
This is a read-only plan for gitops/argo/intermediate-git-as-database.
Nothing will be mutated.

Conceptual model:

  repo/appsets/mon.yaml     (ApplicationSet: 6 cluster generators, 3 versions)
  repo/appsets/canary.yaml  (ApplicationSet: 1 cluster)
    -> repo/clusters/*.yaml (24 targets, key encoded in the filename)
    <- repo/values/**       (layered values, some layers never resolve)

This example will:
- run python3 prompts/scout/scout.py over repo/
- print one section per question, with the numbers scout computed

Mutations if you run without --explain:
- none: no ConfigHub calls, no cluster calls, no file writes

The expected answers are in EXPECTED.md. ./verify.sh checks them.
EOF_PLAN
  exit 0
fi

if [[ "$EXPLAIN_JSON" -eq 1 ]]; then
  cat <<'EOF_JSON'
{
  "example_name": "gitops-argo-intermediate-git-as-database",
  "mutates": false,
  "mutates_confighub": false,
  "mutates_live_infra": false,
  "spaces": [],
  "units": [],
  "inputs": ["repo/"],
  "tool": "prompts/scout/scout.py",
  "questions": [
    "inputs approved, outputs deployed",
    "primary key is a filename",
    "promotion by find-and-replace",
    "in progress, or abandoned",
    "one value edited, how many changed",
    "safer, or just rarer",
    "allowed to differ, or just differing"
  ],
  "evaluation_modes": {
    "fast_preview": {
      "mutates": false,
      "commands": ["./setup.sh --explain", "./setup.sh --explain-json | jq"]
    },
    "fast_operational_evaluation": {
      "mutates_confighub": false,
      "mutates_live_infra": false,
      "commands": ["./setup.sh", "./verify.sh"]
    }
  }
}
EOF_JSON
  exit 0
fi

command -v python3 >/dev/null 2>&1 || { echo "Missing required command: python3" >&2; exit 1; }
python3 -c 'import yaml' 2>/dev/null || { echo "scout needs pyyaml: pip install pyyaml" >&2; exit 1; }
python3 "$SCOUT" "$REPO_DIR"
