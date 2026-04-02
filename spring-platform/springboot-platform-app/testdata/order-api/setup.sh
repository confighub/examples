#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUMMARY_JSON="$ROOT_DIR/example-summary.json"

case "${1:-}" in
  --explain)
    cat <<'EOF'
================================================================================
                    PLAIN CONFIGHUB / GENERATOR VIEW
================================================================================

This is the core Spring platform example showing how cub-gen transforms
app inputs + platform inputs into the Deployment, ConfigMap, and Service
that ConfigHub manages for this app.

THE MODEL:
  App inputs (upstream/app/)
    + Platform inputs (upstream/platform/)
    → Generator (generator/render.sh)
    → Operational outputs (operational/, confighub/)
    → ConfigHub (units across dev, stage, prod)

WHAT THIS SHOWS:
  - How generator produces field lineage and mutation routes
  - How ConfigHub stores the rendered operational state for each environment
  - How field ownership determines what can be mutated where
  - How changes flow: apply-here, lift-upstream, or block/escalate

TARGET MODES:
  - ConfigHub-only: ./confighub-setup.sh (no cluster)
  - Noop targets: ./confighub-setup.sh --with-noop-targets (simulation)
  - Real Kubernetes: ./confighub-setup.sh --with-targets (requires cluster)

NEXT STEPS:
  ./verify.sh                        Check fixture consistency
  ./confighub-setup.sh --explain     Preview ConfigHub setup
  ./generator/render.sh --explain    See generator transformation

See README.md for the full model explanation.
================================================================================
EOF
    ;;
  --explain-json)
    cat "$SUMMARY_JSON"
    ;;
  *)
    cat <<'EOF' >&2
Usage: ./setup.sh [--explain|--explain-json]

  --explain           Human-readable preview of the generator view
  --explain-json      Machine-readable example contract

For ConfigHub operations:
  ./confighub-setup.sh --explain    Preview ConfigHub setup
  ./confighub-setup.sh              Create ConfigHub objects (mutating)
EOF
    exit 2
    ;;
esac
