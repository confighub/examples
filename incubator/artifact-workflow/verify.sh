#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

if [[ ! -f "$OUTPUT_DIR/bundle-inspect.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

jq -e '.target.kind == "Deployment" and .target.name == "web-frontend" and .target.namespace == "production"' "$OUTPUT_DIR/bundle-inspect.json" >/dev/null
jq -e '.contents.hasDrift == true and .contents.hasSession == true and .contents.driftCount == 1' "$OUTPUT_DIR/bundle-inspect.json" >/dev/null
jq -e '.command == "bundle replay"' "$OUTPUT_DIR/bundle-replay-drift.json" >/dev/null
jq -e '.summary.totalFindings == 1 and .summary.bySeverity[0].severity == "warning" and .summary.bySeverity[0].count == 1' "$OUTPUT_DIR/bundle-replay-drift.json" >/dev/null
jq -e '.findings[0].object.kind == "Deployment" and .findings[0].object.name == "web-frontend" and .findings[0].severity == "warning"' "$OUTPUT_DIR/bundle-replay-drift.json" >/dev/null
jq -e '.target == "Deployment/web-frontend"' "$OUTPUT_DIR/bundle-summarize.json" >/dev/null
jq -e '.gitContext.branch == "main" and .gitContext.commit == "abc123def456789"' "$OUTPUT_DIR/bundle-summarize.json" >/dev/null
jq -e '.changes.driftCount == 1 and .riskSignals[0].level == "warning"' "$OUTPUT_DIR/bundle-summarize.json" >/dev/null

echo "Artifact workflow checks passed"
