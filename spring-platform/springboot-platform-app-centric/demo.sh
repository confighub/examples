#!/usr/bin/env bash
# Demo the three mutation outcomes for springboot-platform-app-centric
#
# Usage:
#   ./demo.sh                  # Show all three outcomes
#   ./demo.sh apply-here       # Demo apply-here only
#   ./demo.sh lift-upstream    # Demo lift-upstream only
#   ./demo.sh block-escalate   # Demo block-escalate only

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CUB="${CUB:-cub}"

show_apply_here() {
  cat <<'EOF'
================================================================================
                           APPLY HERE
================================================================================

Scenario:
  Change feature.inventory.reservationMode from 'strict' to 'optimistic'
  for a gradual rollout in production.

Why this applies here:
  - Field pattern: feature.inventory.*
  - Owner: app-team
  - Route: mutable-in-ch (direct ConfigHub mutation)

The change:
  - Stored in ConfigHub for inventory-api-prod
  - Survives future refreshes (policy-driven merge)
  - Can be applied to target immediately

EOF

  if command -v "${CUB}" >/dev/null 2>&1; then
    cat <<'EOF'
Try it:
  cub function do --space inventory-api-prod --unit inventory-api \
    set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic

  cub unit apply --space inventory-api-prod inventory-api

EOF
  fi

  cat <<'EOF'
See: flows/apply-here.md

Field routing rules: ../shared/field-routes.yaml
================================================================================
EOF
}

show_lift_upstream() {
  cat <<'EOF'
================================================================================
                           LIFT UPSTREAM
================================================================================

Scenario:
  Add Redis-backed caching to the service.

Why this lifts upstream:
  - Field pattern: spring.cache.*
  - Owner: app-team
  - Route: lift-upstream (change goes back to app source)

The change:
  - App code needs a Redis dependency (pom.xml)
  - Spring config needs cache settings (application.yaml)
  - These are upstream inputs, not operational mutations

What happens:
  - ConfigHub captures the intent
  - A lift-upstream bundle is produced (automated PR creation is not implemented yet)
  - Developer manually creates a PR with the bundle contents
  - After merge, platform re-renders operational config
  - ConfigHub refreshes from the new rendered state

See: flows/lift-upstream.md

Field routing rules: ../shared/field-routes.yaml
For bundle details, see the core example: springboot-platform-app/lift-upstream/
================================================================================
EOF
}

show_block_escalate() {
  cat <<'EOF'
================================================================================
                           BLOCK / ESCALATE
================================================================================

Scenario:
  Attempt to change spring.datasource.* or bypass the managed datasource.

Why this is blocked:
  - Field pattern: spring.datasource.*
  - Owner: platform-engineering
  - Route: generator-owned (not safe for app-local divergence)

The change:
  - Datasource connectivity is platform-controlled
  - Direct mutation would bypass runtime policy
  - App team cannot mutate this field directly

What happens:
  - The boundary is documented and previewed
  - Server-side enforcement is not implemented yet
  - In production, platform engineer would be notified (escalation)
  - Change requires platform approval or platform path

See: flows/block-escalate.md

Field routing rules: ../shared/field-routes.yaml
================================================================================
EOF
}

show_all() {
  show_apply_here
  echo ""
  show_lift_upstream
  echo ""
  show_block_escalate
}

case "${1:-all}" in
  apply-here)
    show_apply_here
    ;;
  lift-upstream)
    show_lift_upstream
    ;;
  block-escalate)
    show_block_escalate
    ;;
  all)
    show_all
    ;;
  *)
    echo "Usage: $0 [apply-here|lift-upstream|block-escalate|all]" >&2
    exit 2
    ;;
esac
