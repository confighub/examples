#!/usr/bin/env bash
set -euo pipefail

show_explain() {
  cat <<'EOF'
block-escalate: boundary bundle for springboot-platform-app

Proof type: block-escalate-boundary

This is a read-only boundary bundle for the "block/escalate" route.

What it proves:
- `spring.datasource.*` is marked generator-owned in the route rules
- the runtime policy declares a managed datasource boundary
- the exact dry-run mutation command is known for a datasource override attempt

What it does NOT prove:
- actual field-level blocking in ConfigHub
- approval or escalation workflow execution

Read-only commands:
- ./block-escalate.sh --explain-json | jq
- ./block-escalate.sh --render-attempt
- ./block-escalate-verify.sh
EOF
}

show_explain_json() {
  cat <<'EOF'
{
  "example_name": "springboot-platform-app",
  "proof_type": "block-escalate-boundary",
  "mutates_confighub": false,
  "mutates_live_infra": false,
  "current_status": "not_proven",
  "route_rule": "spring.datasource.*",
  "owner": "platform-engineering",
  "attempted_env_key": "SPRING_DATASOURCE_URL",
  "attempted_change_desc": "block-escalate: attempt datasource override for prod",
  "render_attempt": "./block-escalate.sh --render-attempt",
  "verify": "./block-escalate-verify.sh"
}
EOF
}

render_attempt() {
  cat <<'EOF'
# This is the exact dry-run mutation the example treats as platform-owned.
# It should eventually be blocked or escalated by field policy.
# Today it is a concrete boundary artifact, not a proof of enforcement.

cub function do \
  --space order-api-prod \
  --unit order-api \
  --dry-run \
  --change-desc "block-escalate: attempt datasource override for prod" \
  -- \
  set-env order-api "SPRING_DATASOURCE_URL=jdbc:postgresql://rogue.example:5432/inventory"
EOF
}

case "${1:-}" in
  --explain)
    show_explain
    ;;
  --explain-json)
    show_explain_json
    ;;
  --render-attempt)
    render_attempt
    ;;
  *)
    cat <<'EOF' >&2
Use one of:
  ./block-escalate.sh --explain
  ./block-escalate.sh --explain-json
  ./block-escalate.sh --render-attempt
EOF
    exit 2
    ;;
esac
