#!/usr/bin/env bash
# ConfigHub setup for springboot-platform-app
#
# Creates ConfigHub spaces and units for inventory-api across dev, stage, prod.
#
# Usage:
#   ./confighub-setup.sh --explain              # Human-readable preview (read-only)
#   ./confighub-setup.sh --explain-json         # Machine-readable preview (read-only)
#   ./confighub-setup.sh                        # ConfigHub-only (spaces + units)
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
  cat <<EOT
confighub-setup: springboot-platform-app

Mode: confighub-only

What it creates:
- 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
- 1 unit per space: inventory-api (ConfigMap + Deployment + Service)
- labels: ExampleName=springboot-platform-app, App=inventory-api, Environment=<env>

This does not create targets or deliver anything to a cluster.

Cleanup:
- ./confighub-cleanup.sh
EOT
}

# --explain-json: machine-readable preview
show_explain_json() {
  cat <<ENDJSON
{
  "example_name": "springboot-platform-app",
  "proof_type": "confighub-only",
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "requires_cluster": false,
  "with_targets": false,
  "spaces_created": [
    "inventory-api-dev",
    "inventory-api-stage",
    "inventory-api-prod"
  ],
  "units_per_space": ["inventory-api"],
  "cleanup": "./confighub-cleanup.sh"
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
  --with-targets|--with-noop-targets)
    echo "error: ${1} was removed: this example no longer creates targets or delivers to a cluster." >&2
    exit 2
    ;;
  "")
    ;;
  *)
    echo "Usage: $0 [--explain|--explain-json]" >&2
    exit 2
    ;;
esac

# Mutating path: require cub
command -v "${CUB}" >/dev/null 2>&1 || {
  echo "error: cub CLI not found. Install cub and run cub auth login first." >&2
  exit 1
}

echo "=== ConfigHub-only setup for springboot-platform-app ==="
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

# Phase 2: Create units from YAML
echo "Phase 2: Creating units from operational YAML..."

for env in "${ENVS[@]}"; do
  space="$(space_name "${env}")"
  yaml_file="${SCRIPT_DIR}/confighub/inventory-api-${env}.yaml"

  if [[ ! -f "${yaml_file}" ]]; then
    echo "  error: missing ${yaml_file}" >&2
    exit 1
  fi

  # Delete existing unit first to ensure fresh YAML content is used
  # (--allow-exists does not update existing content)
  if ${CUB} unit get --space "${space}" inventory-api >/dev/null 2>&1; then
    echo "y" | ${CUB} unit delete --space "${space}" inventory-api >/dev/null 2>&1 || true
  fi

  ${CUB} unit create --space "${space}" inventory-api "${yaml_file}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --label "Component=backend" \
    --quiet
  echo "  Created unit: ${space}/inventory-api"
done

echo "  Done."
echo ""
echo "=== Setup complete ==="
echo ""
echo "Inspect with:"
echo "  ${CUB} space list --where \"Labels.ExampleName = '${EXAMPLE_LABEL}'\" --json"
echo "  ${CUB} unit get --space inventory-api-prod --json inventory-api"
echo ""
echo "Clean up with:"
echo "  ./confighub-cleanup.sh"
