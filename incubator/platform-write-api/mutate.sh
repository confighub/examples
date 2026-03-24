#!/usr/bin/env bash
# The write API: mutate a config field in ConfigHub.
#
# This is the core proof. Instead of:
#   clone → branch → find YAML → edit field → commit → push → PR → merge → sync
# You do:
#   cub function do set-env <unit> <KEY=VALUE>
#
# Usage:
#   ./mutate.sh              # Run the mutation
#   ./mutate.sh --explain    # What this does (read-only, side-by-side visual)
#   ./mutate.sh --json       # Machine-readable explanation
#   ./mutate.sh --dry-run    # Show the command without executing
#   ./mutate.sh --revert     # Revert to the original value

set -euo pipefail

CUB="${CUB:-cub}"
SPACE="inventory-api-prod"
UNIT="inventory-api"

show_explain() {
  cat <<'EOF'
platform-write-api mutate

Demonstrates the "write API" for platform config.

┌─ GitOps (8 steps) ─────────────────────┬─ ConfigHub (1 command) ──────────────┐
│                                         │                                      │
│  ☐ git clone infra-repo                 │  ✓ cub function do \                 │
│  ☐ git checkout -b feature/rollout      │      --space inventory-api-prod \    │
│  ☐ find overlays/prod/kustomization...  │      --change-desc "rollout" \       │
│  ☐ vim values.yaml                      │      -- set-env inventory-api \      │
│  ☐ git add && git commit                │      "RESERVATION_MODE=optimistic"   │
│  ☐ git push && gh pr create             │                                      │
│  ☐ Wait for CI + review                 │  Time:       ~2 seconds              │
│  ☐ Merge → wait for sync                │  Audit:      automatic               │
│                                         │  Reversible: ./mutate.sh --revert    │
│  Time:  15–45 minutes                   │                                      │
│  Audit: buried in git log               │                                      │
└─────────────────────────────────────────┴──────────────────────────────────────┘

The mutation:
- Field: feature.inventory.reservationMode
- From:  strict (prod upstream default)
- To:    optimistic (rollout override)
- Route: mutable-in-ch (safe to edit directly)

What gets recorded:
- The change description
- Who made it
- When
- The before/after values
- The revision number

What does NOT happen:
- No Git commit
- No CI pipeline
- No Helm chart bump
- No cluster restart (that needs a target, which this example skips)
EOF
}

show_explain_json() {
  cat <<'EOF'
{
  "action": "mutate",
  "space": "inventory-api-prod",
  "unit": "inventory-api",
  "field": "feature.inventory.reservationMode",
  "envVar": "FEATURE_INVENTORY_RESERVATIONMODE",
  "from": "strict",
  "to": "optimistic",
  "route": "mutable-in-ch",
  "gitopsSteps": 8,
  "confighubSteps": 1,
  "readOnly": false,
  "reversible": true,
  "reverseCommand": "./mutate.sh --revert"
}
EOF
}

case "${1:-}" in
  --explain) show_explain; exit 0 ;;
  --explain-json|--json) show_explain_json; exit 0 ;;
  --dry-run)
    echo "Would run:"
    echo ""
    echo "  cub function do --space ${SPACE} \\"
    echo "    --change-desc 'apply-here: reservation mode rollout' \\"
    echo "    -- set-env ${UNIT} 'FEATURE_INVENTORY_RESERVATIONMODE=optimistic'"
    echo ""
    echo "This sets the FEATURE_INVENTORY_RESERVATIONMODE env var on the"
    echo "inventory-api Deployment in the inventory-api-prod space."
    echo "Spring Boot relaxed binding maps this to feature.inventory.reservationMode."
    exit 0
    ;;
  --revert)
    echo "Reverting reservation mode to upstream default (strict)..."
    ${CUB} function do --space "${SPACE}" \
      --change-desc "revert: reservation mode back to strict" \
      -- set-env "${UNIT}" "FEATURE_INVENTORY_RESERVATIONMODE=strict" \
      --quiet 2>/dev/null || true
    echo "Done."
    echo ""
    echo "Verify: ./compare.sh"
    exit 0
    ;;
  "") ;;
  *) echo "Usage: $0 [--explain|--json|--dry-run|--revert]" >&2; exit 2 ;;
esac

command -v "${CUB}" >/dev/null 2>&1 || { echo "error: cub CLI not found." >&2; exit 1; }

echo "=== Platform Write API — the mutation ==="
echo ""
echo "Changing feature.inventory.reservationMode for ${SPACE}:"
echo "  strict → optimistic"
echo ""

# Show before
echo "Before:"
${CUB} unit get --space "${SPACE}" --data-only --json "${UNIT}" 2>/dev/null | \
  jq -r '[.[] | select(.kind == "Deployment")
    | .spec.template.spec.containers[]?.env[]?
    | select(.name == "FEATURE_INVENTORY_RESERVATIONMODE")
    | "  FEATURE_INVENTORY_RESERVATIONMODE = \(.value)"] | first // "  (not set — using ConfigMap default: strict)"' 2>/dev/null || \
  echo "  (could not read current value)"

echo ""

# Mutate
echo "Mutating..."
${CUB} function do --space "${SPACE}" \
  --change-desc "apply-here: reservation mode rollout (strict → optimistic)" \
  -- set-env "${UNIT}" "FEATURE_INVENTORY_RESERVATIONMODE=optimistic" \
  --quiet 2>/dev/null || true

echo ""

# Show after
echo "After:"
${CUB} unit get --space "${SPACE}" --data-only --json "${UNIT}" 2>/dev/null | \
  jq -r '[.[] | select(.kind == "Deployment")
    | .spec.template.spec.containers[]?.env[]?
    | select(.name == "FEATURE_INVENTORY_RESERVATIONMODE")
    | "  FEATURE_INVENTORY_RESERVATIONMODE = \(.value)"] | first // "  (not set)"' 2>/dev/null || \
  echo "  (could not read new value)"

echo ""

# Show mutation history
echo "Mutation history (last 3):"
${CUB} mutation list --space "${SPACE}" --json "${UNIT}" 2>/dev/null | \
  jq -r '.[-3:][] | "  #\(.MutationNum) \(.CreatedAt | split("T")[0]) \(.Description // "no description")"' 2>/dev/null || \
  echo "  (mutation list not available)"

echo ""
echo "That was the write API."
echo ""
echo "Next:"
echo "  ./compare.sh           — see the divergence marker on prod"
echo "  ./refresh-preview.sh   — see that this mutation would survive a refresh"
echo "  ./mutate.sh --revert   — revert to the upstream default"
