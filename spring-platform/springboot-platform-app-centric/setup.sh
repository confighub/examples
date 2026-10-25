#!/usr/bin/env bash
# Setup for springboot-platform-app-centric (ADT view)
#
# Creates ConfigHub spaces and units for inventory-api across dev, stage, prod.
# This example presents the App → Deployments → Targets view of the platform model.
#
# Usage:
#   ./setup.sh --explain              # Show the ADT view (read-only)
#   ./setup.sh --explain-json         # Show the setup plan as JSON (read-only)
#   ./setup.sh                        # Create spaces and units (no cluster needed)
#
# --confighub-only is still accepted and means the same as no flag.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="${SCRIPT_DIR}/../shared"
CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app-centric"

ENVS=(dev stage prod)

space_name() {
  echo "inventory-api-${1}"
}

show_adt_view() {
  cat <<'EOF'
================================================================================
                        APP - DEPLOYMENT - TARGET VIEW
================================================================================

APP: inventory-api
  A Spring Boot inventory service with feature flags and runtime tuning.
  Source: inventory-api Spring Boot application

DEPLOYMENTS:
  +------------------+----------------------+--------------------------+
  | Deployment       | ConfigHub Space      | Purpose                  |
  +------------------+----------------------+--------------------------+
  | dev              | inventory-api-dev    | Development iteration    |
  | stage            | inventory-api-stage  | Validation before prod   |
  | prod             | inventory-api-prod   | Production workload      |
  +------------------+----------------------+--------------------------+

EOF
  cat <<'EOT'
TARGETS:
  No targets. Units exist in ConfigHub only.
  Delivery to a cluster is out of scope for this example: ConfigHub
  delivers Releases to clusters through Argo CD or Flux.

EOT

  cat <<'EOF'
MUTATION OUTCOMES:
  +------------------+---------------------------+----------------------+
  | Outcome          | Example Field             | Owner                |
  +------------------+---------------------------+----------------------+
  | Apply here       | feature.inventory.*       | app-team             |
  | Lift upstream    | spring.cache.*            | app-team             |
  | Block/escalate   | spring.datasource.*       | platform-engineering |
  +------------------+---------------------------+----------------------+

  Field routing rules: ../shared/field-routes.yaml

================================================================================
EOF
}

show_explain() {
  show_adt_view

  cat <<'EOF2'

What this setup does:
  - Creates 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
  - Creates 1 unit per space: inventory-api
  - Does NOT create targets or deliver to a cluster

Cluster required: No
Mutates ConfigHub: Yes
Mutates live infrastructure: No

Safe next steps:
  ./setup.sh                # Create spaces and units
  ./verify.sh               # Verify consistency
  ./cleanup.sh              # Remove all example objects
EOF2
}

show_explain_json() {
  cat <<'EOF2'
{
  "example_name": "springboot-platform-app-centric",
  "proof_type": "adt-view",
  "selected_mode": "confighub-only",
  "default_mode": "confighub-only",
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "cluster_required": false,
  "creates_targets": false,
  "deployment_map": "./deployment-map.json",
  "evaluation_modes": {
    "fast_preview": {
      "mutates": false,
      "commands": [
        "./setup.sh --explain",
        "./setup.sh --explain-json | jq",
        "./demo.sh"
      ],
      "proves": [
        "the ADT model",
        "the configured mutation outcomes",
        "the planned object layout"
      ]
    },
    "fast_operational_evaluation": {
      "mutates_confighub": true,
      "mutates_live_infra": false,
      "commands": [
        "./setup.sh",
        "cub space list --where \"Labels.ExampleName = 'springboot-platform-app-centric'\" --json | jq '.[].Space.Slug'",
        "./verify.sh",
        "./demo.sh",
        "cub function do --space inventory-api-prod --unit inventory-api --change-desc \"demo: reservation mode strict -> optimistic\" set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic",
        "cub mutation list --space inventory-api-prod --json inventory-api | jq '.[] | {mutationNum: .Mutation.MutationNum, source: .Mutation.Source, description: .Revision.Description, createdAt: .Mutation.CreatedAt, author: .Author.Email}'"
      ],
      "stop_before_cleanup": true,
      "proves": [
        "setup succeeds",
        "objects can be isolated by ExampleName",
        "the three mutation outcomes are explained with stable CLI output",
        "one representative apply-here mutation works",
        "mutation history is captured"
      ]
    }
  },
  "next_steps": [
    "./setup.sh",
    "./verify.sh",
    "./cleanup.sh"
  ]
}
EOF2
}

# Parse arguments
EXPLAIN=false
EXPLAIN_JSON=false

while [[ $# -gt 0 ]]; do
  case "${1}" in
    --explain)
      EXPLAIN=true
      shift
      ;;
    --explain-json)
      EXPLAIN_JSON=true
      shift
      ;;
    --confighub-only)
      shift
      ;;
    --with-targets)
      echo "error: --with-targets was removed: this example no longer creates targets or delivers to a cluster." >&2
      exit 2
      ;;
    *)
      echo "Usage: $0 [--explain|--explain-json]" >&2
      exit 2
      ;;
  esac
done

if [[ "${EXPLAIN}" == "true" ]]; then
  show_explain
  exit 0
fi

if [[ "${EXPLAIN_JSON}" == "true" ]]; then
  show_explain_json
  exit 0
fi

# Require cub
command -v "${CUB}" >/dev/null 2>&1 || {
  echo "error: cub CLI not found. Install cub and run cub auth login first." >&2
  exit 1
}

# Check shared YAML files exist
for env in "${ENVS[@]}"; do
  yaml_file="${SHARED_DIR}/confighub/inventory-api-${env}.yaml"
  if [[ ! -f "${yaml_file}" ]]; then
    echo "error: missing ${yaml_file}" >&2
    exit 1
  fi
done

echo "=== ADT setup (ConfigHub-only) ==="
echo ""
echo "All entities are labeled ExampleName=${EXAMPLE_LABEL} for easy cleanup."
echo ""

# Phase 1: Create spaces
echo "Phase 1: Creating spaces..."

for env in "${ENVS[@]}"; do
  space="$(space_name "${env}")"
  ${CUB} space create "${space}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --allow-exists \
    --quiet
  echo "  Created space: ${space}"
done

echo "  Done."
echo ""

# Phase 2: Create units from shared YAML
echo "Phase 2: Creating units..."

for env in "${ENVS[@]}"; do
  space="$(space_name "${env}")"
  yaml_file="${SHARED_DIR}/confighub/inventory-api-${env}.yaml"

  # Delete existing unit first to ensure fresh YAML content is used
  if ${CUB} unit get --space "${space}" inventory-api >/dev/null 2>&1; then
    echo "y" | ${CUB} unit delete --space "${space}" inventory-api >/dev/null 2>&1 || true
  fi

  ${CUB} unit create --space "${space}" inventory-api "${yaml_file}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --quiet
  echo "  Created unit: ${space}/inventory-api"
done

echo "  Done."
echo ""

echo "=== Setup complete ==="
echo ""
echo "Inspect with:"
echo "  ${CUB} space list --where \"Labels.ExampleName = '${EXAMPLE_LABEL}'\" --json | jq '.[].Space.Slug'"
echo "  ${CUB} unit get --space inventory-api-prod --json inventory-api"
echo ""
echo "Clean up with:"
echo "  ./cleanup.sh"
