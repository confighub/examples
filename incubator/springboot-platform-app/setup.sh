#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUMMARY_JSON="$ROOT_DIR/example-summary.json"

case "${1:-}" in
  --explain)
    cat <<'EOF'
springboot-platform-app

This is a structural, read-only incubator example.

Stack:
- Spring Boot app inputs
- platform runtime policy
- ConfigHub as authority for operational config
- provenance back to upstream producers

What it proves:
- one app, inventory-api
- one materialized operational shape
- one upstream Spring Boot API that can be tested over HTTP
- one mutation system with three outcomes:
  - apply here
  - lift upstream
  - block/escalate

What it reads:
- upstream/app/*
- upstream/platform/*
- operational/*
- example-summary.json

What it writes:
- nothing

Safe next step:
- run ./setup.sh --explain-json | jq
- then run ./verify.sh
- optional local app proof: cd upstream/app && mvn test
EOF
    ;;
  --explain-json)
    cat "$SUMMARY_JSON"
    ;;
  *)
    cat <<'EOF' >&2
This example is structural proof only.

Use one of:
  ./setup.sh --explain
  ./setup.sh --explain-json
EOF
    exit 2
    ;;
esac
