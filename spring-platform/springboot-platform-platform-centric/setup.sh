#!/usr/bin/env bash
# Setup for springboot-platform-platform-centric
#
# Creates ConfigHub objects for the springboot-platform with multiple apps.
#
# Usage:
#   ./setup.sh --explain       Human-readable preview (read-only)
#   ./setup.sh --explain-json  Machine-readable preview (read-only)
#   ./setup.sh                 Create all spaces and units (noop targets)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SHARED_DIR="${SCRIPT_DIR}/../shared"
CUB="${CUB:-cub}"
EXAMPLE_LABEL="springboot-platform-platform-centric"

show_explain() {
  cat <<'EOF'
================================================================================
           SPRINGBOOT-PLATFORM (Platform-Centric View)
================================================================================

One platform. Multiple apps. Shared policies.

PLATFORM: springboot-platform
-----------------------------
Provides:
  - managed-datasource: PostgreSQL with HA, encryption, backups
  - runtime-hardening: runAsNonRoot, mTLS sidecar
  - observability: Health endpoints, SLO targets

Controls (blocked fields):
  - spring.datasource.*  → platform boundary
  - securityContext.*    → runtime hardening

APPS ON THIS PLATFORM
---------------------
┌─────────────────┬──────────────────────────────────┬─────────────────────┐
│ App             │ Deployments                      │ Spaces              │
├─────────────────┼──────────────────────────────────┼─────────────────────┤
│ inventory-api   │ dev, stage, prod                 │ inventory-api-*     │
│ catalog-api     │ dev, prod                        │ catalog-api-*       │
└─────────────────┴──────────────────────────────────┴─────────────────────┘

WHAT THIS CREATES
-----------------
- 1 infra space: springboot-platform-infra (server worker)
- 5 app spaces: inventory-api-{dev,stage,prod}, catalog-api-{dev,prod}
- 5 units: one per space
- 5 noop targets: apply workflow works without a cluster

TARGET MODES
------------
Default: noop (apply workflow works, no real cluster needed)

All objects are labeled: ExampleName=springboot-platform-platform-centric

CLEANUP
-------
./cleanup.sh

================================================================================
EOF
}

show_explain_json() {
  cat "${SCRIPT_DIR}/platform-map.json"
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
  "")
    # Continue to setup
    ;;
  *)
    echo "Usage: $0 [--explain|--explain-json]" >&2
    exit 2
    ;;
esac

# Require cub
command -v "${CUB}" >/dev/null 2>&1 || {
  echo "error: cub CLI not found. Install cub and run cub auth login first." >&2
  exit 1
}

echo "=== Setting up springboot-platform (platform-centric) ==="
echo ""
echo "Platform: springboot-platform"
echo "Apps: inventory-api, catalog-api"
echo ""

# Phase 1: Create infra space with server worker
echo "Phase 1: Creating infra space..."
${CUB} space create "springboot-platform-infra" \
  --label "ExampleName=${EXAMPLE_LABEL}" \
  --label "Platform=springboot-platform" \
  --label "Role=infra" \
  --allow-exists \
  --quiet
echo "  Created: springboot-platform-infra"

${CUB} worker create worker --space "springboot-platform-infra" --quiet --is-server-worker \
  --allow-exists 2>/dev/null || true
echo "  Created: springboot-platform-infra/worker"
echo ""

# Phase 2: Create inventory-api spaces and units (delegate to parent)
echo "Phase 2: Creating inventory-api (3 deployments)..."
for env in dev stage prod; do
  space="inventory-api-${env}"
  yaml_file="${SHARED_DIR}/confighub/inventory-api-${env}.yaml"

  ${CUB} space create "${space}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "Platform=springboot-platform" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --allow-exists \
    --quiet

  if ${CUB} unit get --space "${space}" inventory-api >/dev/null 2>&1; then
    echo "y" | ${CUB} unit delete --space "${space}" inventory-api >/dev/null 2>&1 || true
  fi

  ${CUB} unit create --space "${space}" inventory-api "${yaml_file}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "Platform=springboot-platform" \
    --label "App=inventory-api" \
    --label "Environment=${env}" \
    --quiet
  echo "  Created: ${space}/inventory-api"

  # Create noop target
  ${CUB} target create "${env}" '{}' "springboot-platform-infra/worker" -p Noop \
    --space "${space}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --allow-exists \
    --quiet

  ${CUB} unit set-target "${env}" --space "${space}" --unit inventory-api --quiet
done
echo ""

# Phase 3: Create catalog-api spaces and units (minimal app)
echo "Phase 3: Creating catalog-api (2 deployments)..."
for env in dev prod; do
  space="catalog-api-${env}"

  ${CUB} space create "${space}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "Platform=springboot-platform" \
    --label "App=catalog-api" \
    --label "Environment=${env}" \
    --allow-exists \
    --quiet

  if ${CUB} unit get --space "${space}" catalog-api >/dev/null 2>&1; then
    echo "y" | ${CUB} unit delete --space "${space}" catalog-api >/dev/null 2>&1 || true
  fi

  # Create minimal catalog-api unit (simplified version of inventory-api)
  ${CUB} unit create --space "${space}" catalog-api "${SCRIPT_DIR}/apps/catalog-api/${env}.yaml" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --label "Platform=springboot-platform" \
    --label "App=catalog-api" \
    --label "Environment=${env}" \
    --quiet
  echo "  Created: ${space}/catalog-api"

  # Create noop target
  ${CUB} target create "${env}" '{}' "springboot-platform-infra/worker" -p Noop \
    --space "${space}" \
    --label "ExampleName=${EXAMPLE_LABEL}" \
    --allow-exists \
    --quiet

  ${CUB} unit set-target "${env}" --space "${space}" --unit catalog-api --quiet
done
echo ""

# Phase 4: Apply all units
echo "Phase 4: Applying all units..."
for space in inventory-api-dev inventory-api-stage inventory-api-prod; do
  ${CUB} unit apply --space "${space}" inventory-api --quiet
  echo "  Applied: ${space}/inventory-api"
done
for space in catalog-api-dev catalog-api-prod; do
  ${CUB} unit apply --space "${space}" catalog-api --quiet
  echo "  Applied: ${space}/catalog-api"
done
echo ""

echo "=== Setup complete ==="
echo ""
echo "Platform: springboot-platform"
echo "Apps: inventory-api (3 deployments), catalog-api (2 deployments)"
echo ""
echo "Inspect with:"
echo "  ${CUB} space list --where \"Labels.Platform = 'springboot-platform'\" --json | jq '.[].Space.Slug'"
echo "  ./platform.sh --summary"
echo ""
echo "Clean up with:"
echo "  ./cleanup.sh"
