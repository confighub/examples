#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUMMARY_JSON="$ROOT_DIR/example-summary.json"

case "${1:-}" in
  --explain)
    cat <<'EOF'
springboot-platform-app

This is a structural and locally-runnable incubator example with a
ConfigHub-only proof path.

Stack:
- Spring Boot app inputs
- platform runtime policy
- ConfigHub as authority for operational config
- provenance back to upstream producers

Proof levels:
1. Structural: fixture files and contracts (this script)
2. Local app: mvn test and mvn spring-boot:run
3. ConfigHub-only: ./confighub-setup.sh creates real ConfigHub objects

What it reads:
- upstream/app/*
- upstream/platform/*
- operational/*
- example-summary.json

What this script writes:
- nothing (read-only preview only)

Safe next steps:
- run ./setup.sh --explain-json | jq
- then run ./verify.sh
- optional local app proof: cd upstream/app && mvn test
- ConfigHub-only proof: ./confighub-setup.sh --explain
EOF
    ;;
  --explain-json)
    cat "$SUMMARY_JSON"
    ;;
  *)
    cat <<'EOF' >&2
This example is structural proof by default.

Use one of:
  ./setup.sh --explain           Structural preview
  ./setup.sh --explain-json      Machine-readable contract
  ./confighub-setup.sh --explain ConfigHub-only preview
  ./confighub-setup.sh           Create ConfigHub objects (mutating)
EOF
    exit 2
    ;;
esac
