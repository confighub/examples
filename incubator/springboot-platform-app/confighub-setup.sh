#!/usr/bin/env bash
# ConfigHub-only setup for springboot-platform-app
#
# Creates ConfigHub spaces and units for inventory-api across dev, stage, prod.
# No cluster, target, or worker required.
#
# Usage:
#   ./confighub-setup.sh --explain          # Human-readable preview (read-only)
#   ./confighub-setup.sh --explain-json     # Machine-readable preview (read-only)
#   ./confighub-setup.sh                    # Create ConfigHub objects (mutating)
#
# Cleanup:
#   ./confighub-cleanup.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-app"

ENVS=(dev stage prod)

space_name() {
  echo "inventory-api-${1}"
}

# --explain: human-readable preview
show_explain() {
  cat <<'EOF'
confighub-setup: ConfigHub-only proof for springboot-platform-app

Proof type: confighub-only
This creates real ConfigHub objects. It does not create targets, workers,
or apply to a live cluster.

What it creates:
- 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
- 1 unit per space: inventory-api (ConfigMap + Deployment + Service)
- labels: ExampleName=springboot-platform-app, App=inventory-api, Environment=<env>

What it reads:
- confighub/inventory-api-dev.yaml
- confighub/inventory-api-stage.yaml
- confighub/inventory-api-prod.yaml

What it does NOT create:
- targets
- workers
- cluster bindings

Mutating commands used:
- cub space create
- cub unit create

Read-only inspection after setup:
- cub space list --where "Labels.ExampleName = 'springboot-platform-app'" --json
- cub unit get --space inventory-api-prod --json inventory-api

Cleanup:
- ./confighub-cleanup.sh
EOF
}

# --explain-json: machine-readable preview
show_explain_json() {
  cat <<'ENDJSON'
{
  "example_name": "springboot-platform-app",
  "proof_type": "confighub-only",
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "requires_target": false,
  "requires_worker": false,
  "requires_cluster": false,
  "spaces_created": [
    "inventory-api-dev",
    "inventory-api-stage",
    "inventory-api-prod"
  ],
  "units_per_space": [
    "inventory-api"
  ],
  "labels": {
    "ExampleName": "springboot-platform-app",
    "App": "inventory-api"
  },
  "unit_sources": [
    "confighub/inventory-api-dev.yaml",
    "confighub/inventory-api-stage.yaml",
    "confighub/inventory-api-prod.yaml"
  ],
  "cleanup": "./confighub-cleanup.sh",
  "inspect_commands": [
    "cub space list --where \"Labels.ExampleName = 'springboot-platform-app'\" --json",
    "cub unit get --space inventory-api-dev --json inventory-api",
    "cub unit get --space inventory-api-stage --json inventory-api",
    "cub unit get --space inventory-api-prod --json inventory-api"
  ]
}
ENDJSON
}

case "${1:-}" in
  --explain)
    show_explain
    exit 0
    ;;
  --explain-json)
    show_explain_json
    exit 0
    ;;
esac

# Mutating path: require cub
command -v "${CUB}" >/dev/null 2>&1 || {
  echo "error: cub CLI not found. Install cub and run cub auth login first." >&2
  exit 1
}

command -v jq >/dev/null 2>&1 || {
  echo "error: jq not found." >&2
  exit 1
}

echo "=== ConfigHub-only setup for springboot-platform-app ==="
echo ""
echo "This will create 3 spaces and 3 units in your ConfigHub org."
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

# Phase 2: Create units from YAML
echo "Phase 2: Creating units from operational YAML..."

for env in "${ENVS[@]}"; do
  space="$(space_name "${env}")"
  yaml_file="${SCRIPT_DIR}/confighub/inventory-api-${env}.yaml"

  if [[ ! -f "${yaml_file}" ]]; then
    echo "  error: missing ${yaml_file}" >&2
    exit 1
  fi

  ${CUB} unit create --space "${space}" inventory-api "${yaml_file}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --label "Component=backend" \
    --allow-exists \
    --quiet
  echo "  Created unit: ${space}/inventory-api"
done

echo "  Done."
echo ""

echo "=== ConfigHub-only setup complete ==="
echo ""
echo "Inspect with:"
echo "  ${CUB} space list --where \"Labels.ExampleName = '${EXAMPLE_LABEL}'\" --json"
echo "  ${CUB} unit get --space inventory-api-prod --json inventory-api"
echo ""
echo "Clean up with:"
echo "  ./confighub-cleanup.sh"
