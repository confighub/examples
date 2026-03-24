#!/usr/bin/env bash
# Create ConfigHub spaces and units for the platform-write-api example.
#
# Usage:
#   ./setup.sh --explain       # Preview (read-only)
#   ./setup.sh --explain-json  # Machine-readable preview
#   ./setup.sh                 # Create spaces + units

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SPRING_DIR="${SCRIPT_DIR}/../springboot-platform-app"
CUB="${CUB:-cub}"
EXAMPLE_LABEL="platform-write-api"
ENVS=(dev stage prod)

show_explain() {
  cat <<'EOF'
platform-write-api setup

Creates ConfigHub objects for the inventory-api service:
- 3 spaces: inventory-api-dev, inventory-api-stage, inventory-api-prod
- 1 unit per space: inventory-api (ConfigMap + Deployment + Service)
- All labeled ExampleName=platform-write-api

Uses the same operational YAML fixtures as springboot-platform-app.
No targets, no cluster, no GitOps — just ConfigHub as the config API.

After setup:
- ./compare.sh      — see all three environments side by side
- ./field-routes.sh  — see which fields you can change
- ./mutate.sh        — change a field (the write API!)

Cleanup: ./cleanup.sh
EOF
}

show_explain_json() {
  cat <<'ENDJSON'
{
  "example_name": "platform-write-api",
  "proof_type": "confighub-only",
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "requires_cluster": false,
  "spaces_created": [
    "inventory-api-dev",
    "inventory-api-stage",
    "inventory-api-prod"
  ],
  "units_per_space": ["inventory-api"],
  "cleanup": "./cleanup.sh"
}
ENDJSON
}

case "${1:-}" in
  --explain)      show_explain; exit 0 ;;
  --explain-json) show_explain_json; exit 0 ;;
  "") ;;
  *) echo "Usage: $0 [--explain|--explain-json]" >&2; exit 2 ;;
esac

command -v "${CUB}" >/dev/null 2>&1 || { echo "error: cub CLI not found." >&2; exit 1; }

echo "=== Platform Write API — ConfigHub setup ==="
echo ""

for env in "${ENVS[@]}"; do
  space="inventory-api-${env}"
  yaml="${SPRING_DIR}/confighub/inventory-api-${env}.yaml"

  if [[ ! -f "$yaml" ]]; then
    echo "error: missing ${yaml}" >&2
    echo "This example reuses fixtures from ../springboot-platform-app/confighub/" >&2
    exit 1
  fi

  ${CUB} space create "${space}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --allow-exists --quiet
  echo "  Space: ${space}"

  ${CUB} unit create --space "${space}" inventory-api "${yaml}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --allow-exists --quiet
  echo "  Unit:  ${space}/inventory-api"
done

echo ""
echo "Done. Next steps:"
echo "  ./compare.sh         — see config across all environments"
echo "  ./field-routes.sh    — see which fields you can change"
echo "  ./mutate.sh          — change a field (the write API)"
