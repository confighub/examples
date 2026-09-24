#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTS="$SCRIPT_DIR/../../../prompts/scout/test_scout.py"

command -v python3 >/dev/null 2>&1 || { echo "Missing required command: python3" >&2; exit 1; }
python3 -c 'import yaml' 2>/dev/null || { echo "scout needs pyyaml: pip install pyyaml" >&2; exit 1; }

# test_scout.py ends with one check per planted answer in repo/ (EXPECTED.md).
SCOUT_FLEET="$SCRIPT_DIR/repo" python3 "$TESTS"
echo "All checks passed."
