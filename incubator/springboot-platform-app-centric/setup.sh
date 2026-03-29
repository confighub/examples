#!/usr/bin/env bash
# App-centric setup for springboot-platform-app
#
# This is a wrapper that presents the app/deployment/target story
# and delegates to the underlying springboot-platform-app scripts.
#
# Usage:
#   ./setup.sh --explain              # Show the ADT view (read-only)
#   ./setup.sh --explain --confighub-only   # Explain confighub-only mode
#   ./setup.sh --explain --with-targets     # Explain real-target mode
#   ./setup.sh                         # Default: noop targets (no cluster needed)
#   ./setup.sh --confighub-only        # ConfigHub only (no targets)
#   ./setup.sh --with-targets          # Real Kubernetes (requires cluster + worker)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="${SCRIPT_DIR}/../springboot-platform-app"

show_adt_view() {
  local mode="${1:-noop}"
  cat <<'EOF'
================================================================================
                        APP - DEPLOYMENT - TARGET VIEW
================================================================================

APP: inventory-api
  A Spring Boot inventory service with feature flags and runtime tuning.
  Source: ../springboot-platform-app/upstream/app

DEPLOYMENTS:
  +------------------+----------------------+--------------------------+
  | Deployment       | ConfigHub Space      | Purpose                  |
  +------------------+----------------------+--------------------------+
  | dev              | inventory-api-dev    | Development iteration    |
  | stage            | inventory-api-stage  | Validation before prod   |
  | prod             | inventory-api-prod   | Production workload      |
  +------------------+----------------------+--------------------------+

EOF

  case "${mode}" in
    "noop")
      cat <<'EOF'
TARGETS (noop mode - default):
  +------------------+----------------------+--------------------------+
  | Deployment       | Target               | Delivers to              |
  +------------------+----------------------+--------------------------+
  | dev              | dev (Noop)           | (accepts, no delivery)   |
  | stage            | stage (Noop)         | (accepts, no delivery)   |
  | prod             | prod (Noop)          | (accepts, no delivery)   |
  +------------------+----------------------+--------------------------+

  Noop targets let you exercise the full mutation-to-apply workflow
  without needing a Kubernetes cluster.

EOF
      ;;
    "confighub-only")
      cat <<'EOF'
TARGETS (confighub-only mode):
  No targets. Units exist in ConfigHub only.
  Use this to inspect spaces and units before binding targets.

EOF
      ;;
    "real")
      cat <<'EOF'
TARGETS (real mode):
  +------------------+----------------------+--------------------------+
  | Deployment       | Target               | Delivers to              |
  +------------------+----------------------+--------------------------+
  | dev              | (none)               | ConfigHub only           |
  | stage            | (none)               | ConfigHub only           |
  | prod             | Kubernetes target    | Real cluster namespace   |
  +------------------+----------------------+--------------------------+

  Only prod is bound to a real Kubernetes target.
  Requires: Kind cluster, Docker image, ConfigHub worker.

EOF
      ;;
  esac

  cat <<'EOF'
MUTATION OUTCOMES:
  +------------------+---------------------------+----------------------+
  | Outcome          | Example Field             | Owner                |
  +------------------+---------------------------+----------------------+
  | Apply here       | feature.inventory.*       | app-team             |
  | Lift upstream    | spring.cache.*            | app-team             |
  | Block/escalate   | spring.datasource.*       | platform-engineering |
  +------------------+---------------------------+----------------------+

  Field routing rules: ../springboot-platform-app/operational/field-routes.yaml

================================================================================
EOF
}

show_explain() {
  local mode="${1:-noop}"
  show_adt_view "${mode}"

  cat <<EOF

What this setup does:
EOF

  case "${mode}" in
    "noop")
      cat <<'EOF'
  - Creates 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
  - Creates 1 infra space: inventory-api-infra (server worker)
  - Creates 1 unit per space: inventory-api
  - Creates 1 Noop target per space
  - Binds units to Noop targets
  - Applies all units

Cluster required: No
Mutates ConfigHub: Yes
Mutates live infrastructure: No

EOF
      ;;
    "confighub-only")
      cat <<'EOF'
  - Creates 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
  - Creates 1 unit per space: inventory-api
  - Does NOT create targets or apply

Cluster required: No
Mutates ConfigHub: Yes
Mutates live infrastructure: No

EOF
      ;;
    "real")
      cat <<'EOF'
  - Creates 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
  - Creates 1 unit per space: inventory-api
  - Creates Kubernetes namespace: inventory-api
  - Binds prod unit to real Kubernetes target
  - Applies prod unit (triggers kubectl apply via worker)

Cluster required: Yes
Mutates ConfigHub: Yes
Mutates live infrastructure: Yes (prod only)

Required environment variables:
  WORKER_SPACE=<space-where-worker-lives>
  K8S_TARGET=<target-slug>  (optional if exactly one Kubernetes target exists)

EOF
      ;;
  esac

  cat <<'EOF'
Safe next steps:
  ./setup.sh                # Run with noop targets (default)
  ./verify.sh               # Verify consistency
  ./cleanup.sh              # Remove all example objects

Delegation:
  This script delegates to ../springboot-platform-app/confighub-setup.sh
EOF
}

# Parse arguments
MODE="noop"
EXPLAIN=false

while [[ $# -gt 0 ]]; do
  case "${1}" in
    --explain)
      EXPLAIN=true
      shift
      ;;
    --confighub-only)
      MODE="confighub-only"
      shift
      ;;
    --with-targets)
      MODE="real"
      shift
      ;;
    *)
      echo "Usage: $0 [--explain] [--confighub-only|--with-targets]" >&2
      exit 2
      ;;
  esac
done

if [[ "${EXPLAIN}" == "true" ]]; then
  show_explain "${MODE}"
  exit 0
fi

# Check parent directory exists
if [[ ! -d "${PARENT_DIR}" ]]; then
  echo "error: Parent example not found at ${PARENT_DIR}" >&2
  exit 1
fi

# Delegate to parent
case "${MODE}" in
  "noop")
    echo "Setting up with noop targets (default)..."
    echo ""
    exec "${PARENT_DIR}/confighub-setup.sh" --with-noop-targets
    ;;
  "confighub-only")
    echo "Setting up ConfigHub only (no targets)..."
    echo ""
    exec "${PARENT_DIR}/confighub-setup.sh"
    ;;
  "real")
    echo "Setting up with real Kubernetes targets..."
    echo ""
    exec "${PARENT_DIR}/confighub-setup.sh" --with-targets
    ;;
esac
