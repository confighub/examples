#!/usr/bin/env bash
# Platform discovery commands for springboot-platform
#
# Usage:
#   ./platform.sh --summary              What the platform provides and controls
#   ./platform.sh --apps                 List apps on this platform
#   ./platform.sh --explain-field FIELD  Explain why a field is blocked/mutable

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

show_summary() {
  cat <<'EOF'
================================================================================
                    PLATFORM: springboot-platform
================================================================================

WHAT THE PLATFORM PROVIDES
--------------------------
┌─────────────────────┬──────────────────────────────────────────────────────┐
│ Capability          │ Description                                          │
├─────────────────────┼──────────────────────────────────────────────────────┤
│ managed-datasource  │ PostgreSQL with HA, encryption, automated backups   │
│ runtime-hardening   │ Security defaults (runAsNonRoot, mTLS sidecar)       │
│ observability       │ Health endpoints, SLO: 99.9% avail, p95 < 250ms     │
└─────────────────────┴──────────────────────────────────────────────────────┘

APPS ON THIS PLATFORM
---------------------
┌─────────────────┬─────────────────────────────┬───────────────────────────┐
│ App             │ Deployments                 │ App-Owned Fields          │
├─────────────────┼─────────────────────────────┼───────────────────────────┤
│ inventory-api   │ dev, stage, prod            │ feature.inventory.*       │
│ catalog-api     │ dev, prod                   │ feature.catalog.*         │
└─────────────────┴─────────────────────────────┴───────────────────────────┘

PLATFORM-CONTROLLED FIELDS (blocked for all apps)
--------------------------------------------------
  ✗ spring.datasource.*    Managed datasource boundary
  ✗ securityContext.*      Runtime hardening policy

ESCALATION
----------
For changes to blocked fields:
  Slack: #platform-support
  Email: platform@example.com

See: platform.yaml for full policy details
================================================================================
EOF
}

show_apps() {
  cat <<'EOF'
================================================================================
                    APPS ON springboot-platform
================================================================================

APP: inventory-api
------------------
Description: Inventory management service with feature flags
Source: ../springboot-platform-app/upstream/app

Deployments:
  ┌───────────┬─────────────────────────┬────────────────┐
  │ Name      │ Space                   │ Unit           │
  ├───────────┼─────────────────────────┼────────────────┤
  │ dev       │ inventory-api-dev       │ inventory-api  │
  │ stage     │ inventory-api-stage     │ inventory-api  │
  │ prod      │ inventory-api-prod      │ inventory-api  │
  └───────────┴─────────────────────────┴────────────────┘

App-Owned Fields (mutable-in-ch):
  ✓ feature.inventory.*   Per-deployment rollout tuning

APP: catalog-api
----------------
Description: Product catalog service
Source: ./apps/catalog-api

Deployments:
  ┌───────────┬─────────────────────────┬────────────────┐
  │ Name      │ Space                   │ Unit           │
  ├───────────┼─────────────────────────┼────────────────┤
  │ dev       │ catalog-api-dev         │ catalog-api    │
  │ prod      │ catalog-api-prod        │ catalog-api    │
  └───────────┴─────────────────────────┴────────────────┘

App-Owned Fields (mutable-in-ch):
  ✓ feature.catalog.*     Feature flags for catalog

================================================================================
EOF
}

explain_field() {
  local field="${1:-}"
  if [[ -z "${field}" ]]; then
    echo "Usage: $0 --explain-field <field-pattern>"
    echo "Example: $0 --explain-field spring.datasource.url"
    exit 2
  fi

  echo "================================================================================"
  echo "                    FIELD EXPLANATION: ${field}"
  echo "================================================================================"
  echo ""
  echo "Platform: springboot-platform"
  echo ""

  case "${field}" in
    feature.inventory.*|feature.inventory.*)
      cat <<'EOF'
Field Pattern: feature.inventory.*
Owner: app-team (inventory-api)
Action: mutable-in-ch ✓
Blocked: No

Applies to: inventory-api only

Reason:
  Per-deployment rollout tuning is app-team safe. These fields control
  feature flags specific to inventory-api.

Example:
  cub function do --space inventory-api-prod --unit inventory-api \
    set-env inventory-api FEATURE_INVENTORY_RESERVATIONMODE=optimistic
EOF
      ;;
    feature.catalog.*|feature.catalog.*)
      cat <<'EOF'
Field Pattern: feature.catalog.*
Owner: app-team (catalog-api)
Action: mutable-in-ch ✓
Blocked: No

Applies to: catalog-api only

Reason:
  Per-deployment feature flags for catalog-api.

Example:
  cub function do --space catalog-api-prod --unit catalog-api \
    set-env catalog-api FEATURE_CATALOG_SEARCHENABLED=true
EOF
      ;;
    spring.datasource.*|spring.datasource.*)
      cat <<'EOF'
Field Pattern: spring.datasource.*
Owner: platform-engineering
Action: generator-owned ✗
Blocked: YES

Applies to: ALL apps on springboot-platform

Reason:
  Datasource connectivity is managed by platform for HA and security.
  The platform provides a managed PostgreSQL service with:
  - High availability
  - Encryption at rest
  - Automated backups
  - Connection pooling

What happens if you try to change it:
  The boundary is documented; server-side enforcement is not yet implemented.
  In production, this mutation would be blocked or escalated.
  Contact platform-engineering via #platform-support

Source:
  platform.yaml → spec.fieldRoutes[spring.datasource.*]
EOF
      ;;
    securityContext.*|securityContext.*)
      cat <<'EOF'
Field Pattern: securityContext.*
Owner: platform-engineering
Action: generator-owned ✗
Blocked: YES

Applies to: ALL apps on springboot-platform

Reason:
  Runtime hardening must remain platform-controlled. The platform
  enforces security defaults:
  - runAsNonRoot: true
  - mTLS sidecar injection

What happens if you try to change it:
  The boundary is documented; server-side enforcement is not yet implemented.
  In production, this mutation would be blocked or escalated.
  Contact platform-engineering via #platform-support

Source:
  platform.yaml → spec.fieldRoutes[securityContext.*]
EOF
      ;;
    *)
      cat <<EOF
Field Pattern: ${field}
Status: Unknown

This field is not explicitly listed in the platform policy.

Check platform.yaml for all field routes.

For help:
  Slack: #platform-support
EOF
      ;;
  esac

  echo ""
  echo "================================================================================"
}

case "${1:-}" in
  --summary)
    show_summary
    ;;
  --apps)
    show_apps
    ;;
  --explain-field)
    explain_field "${2:-}"
    ;;
  *)
    echo "Usage: $0 <command>"
    echo ""
    echo "Commands:"
    echo "  --summary              What the platform provides and controls"
    echo "  --apps                 List apps on this platform"
    echo "  --explain-field FIELD  Explain why a field is blocked/mutable"
    echo ""
    echo "Examples:"
    echo "  $0 --summary"
    echo "  $0 --apps"
    echo "  $0 --explain-field spring.datasource.url"
    exit 2
    ;;
esac
