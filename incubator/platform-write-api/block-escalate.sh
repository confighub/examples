#!/usr/bin/env bash
# Scenario 3: Block a mutation on a generator-owned field.
#
# Some fields are owned by the platform — the datasource URL, managed
# credentials, compliance settings. App teams should not override these.
# ConfigHub classifies these as "generator-owned" and blocks direct mutation.
#
# This script demonstrates the blocking behavior and escalation path.
#
# Usage:
#   ./block-escalate.sh                  # Show the block decision
#   ./block-escalate.sh --render-attempt # Simulate a blocked mutation attempt
#   ./block-escalate.sh --explain        # What this demonstrates
#   ./block-escalate.sh --json           # Machine-readable output

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

show_explain() {
  cat <<'EOF'
block-escalate — Scenario 3: Block mutation on platform-owned fields

The story:
  A developer tries to change spring.datasource.url for inventory-api-prod.
  They want to point it at a different database — maybe for testing, maybe
  by mistake. But this is a platform-managed field. The datasource is
  provisioned by the platform team with managed credentials, failover
  configuration, and compliance settings.

Why it must be blocked:
  - The platform team provisions and manages the database
  - Connection strings include managed credentials
  - Compliance and audit requirements apply
  - Letting app teams override would break the managed boundary

What happens today (GitOps-only):
  Nothing stops them. Anyone with Git write access can edit any field in
  any file. There's no field-level governance. The platform team discovers
  the override after something breaks in production.

What ConfigHub gives you:
  1. The field-route model classifies spring.datasource.* as "generator-owned"
  2. Direct mutation is blocked with a clear explanation
  3. An escalation path is provided (platform-team channel)
  4. The attempt is logged for audit

What this proves:
  - The routing model can express field-level governance
  - Every field has an explicit owner and mutation policy
  - The boundary is documented and inspectable

What is NOT yet proven:
  - Server-side enforcement (today blocking is in scripts, not server policy)
  - Automated escalation workflow (no ticketing integration)
  - These are documented gaps — cub-gen #207 tracks server enforcement
EOF
}

show_block() {
  cat <<'EOF'
=== Block/Escalate — Routing Decision ===

Requested change:
  spring.datasource.url: jdbc:postgresql://prod-db:5432/inventory
                       → jdbc:postgresql://my-test-db:5432/inventory

Field route lookup:
  FIELD                   CURRENT                                    ROUTE
  spring.datasource.url   jdbc:postgresql://prod-db:5432/inventory   generator-owned
  spring.datasource.*     —                                          generator-owned

Decision: BLOCKED

  This field is classified as "generator-owned" because the platform team
  provisions and manages the database connection:
    - Managed PostgreSQL instance with automated failover
    - Credentials rotated by platform secrets management
    - Connection pooling and SSL configured by platform
    - Compliance logging enabled at the platform level

  Direct mutation is not allowed. The app team cannot override this field.

Escalation path:
  To request a database change for inventory-api-prod:
    1. Open a ticket in #platform-requests
    2. Or: Create an issue in platform-team/database-requests
    3. Include: service name, environment, reason for change

  The platform team will evaluate the request and, if approved, update
  the generator configuration. The change will flow through the normal
  platform release process.

Audit log entry:
  [BLOCKED] User attempted to change spring.datasource.url
  [REASON]  Field is generator-owned (platform-managed)
  [ACTION]  Escalation path provided
EOF
}

show_attempt() {
  cat <<'EOF'
=== Block/Escalate — Simulated Mutation Attempt ===

$ cub function do --space inventory-api-prod \
    --change-desc "point to test database" \
    -- set-env inventory-api "SPRING_DATASOURCE_URL=jdbc:postgresql://my-test-db:5432/inventory"

--------------------------------------------------------------------------------
MUTATION BLOCKED
--------------------------------------------------------------------------------

FIELD:    spring.datasource.url
ROUTE:    generator-owned
OWNER:    platform-team

ACTION:   BLOCKED — this field is provisioned by the platform.
          Direct mutation is not allowed.

REASON:   The spring.datasource.* fields are managed by the platform team.
          They control database provisioning, credentials, failover, and
          compliance settings. App teams cannot override these values.

ESCALATE: To request a change to this field:
            - Slack: #platform-requests
            - GitHub: platform-team/database-requests/issues/new
            - Include: service name, environment, business justification

--------------------------------------------------------------------------------

What was logged:
  Timestamp:    2024-01-15T14:32:17Z
  User:         developer@company.com
  Space:        inventory-api-prod
  Unit:         inventory-api
  Field:        spring.datasource.url
  Attempted:    jdbc:postgresql://my-test-db:5432/inventory
  Result:       BLOCKED
  Route:        generator-owned
  Owner:        platform-team

This attempt is recorded in the ConfigHub audit log even though the
mutation was blocked. The platform team can see who tried to change
what and when.
EOF
}

show_json() {
  cat <<'ENDJSON'
{
  "scenario": "block-escalate",
  "requested_change": {
    "field": "spring.datasource.url",
    "from": "jdbc:postgresql://prod-db:5432/inventory",
    "to": "jdbc:postgresql://my-test-db:5432/inventory"
  },
  "routing": {
    "route": "generator-owned",
    "owner": "platform-team",
    "blocked": true,
    "reason": "platform_managed_field"
  },
  "escalation": {
    "channels": [
      {"type": "slack", "target": "#platform-requests"},
      {"type": "github", "target": "platform-team/database-requests"}
    ],
    "required_info": ["service_name", "environment", "business_justification"]
  },
  "audit": {
    "logged": true,
    "result": "BLOCKED",
    "visible_to": ["platform-team", "security-team"]
  },
  "enforcement_status": {
    "client_side": "implemented",
    "server_side": "not_implemented",
    "tracking_issue": "cub-gen #207"
  }
}
ENDJSON
}

case "${1:-}" in
  --explain)        show_explain; exit 0 ;;
  --render-attempt) show_attempt; exit 0 ;;
  --json)           show_json; exit 0 ;;
  "")               show_block; exit 0 ;;
  *)                echo "Usage: $0 [--explain|--render-attempt|--json]" >&2; exit 2 ;;
esac
