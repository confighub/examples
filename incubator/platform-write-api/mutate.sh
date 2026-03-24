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
#   ./mutate.sh --explain    # What this does (read-only)
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

The old way (GitOps-only):
  1. Clone the repo
  2. Create a branch
  3. Find the right YAML file
  4. Edit the field (hope you get the indentation right)
  5. Commit and push
  6. Open a PR
  7. Wait for review and merge
  8. Wait for GitOps sync

The ConfigHub way:
  cub function do --space inventory-api-prod \
    --change-desc "apply-here: reservation mode rollout" \
    -- set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"

One command. Immediate. Audited. Reversible.

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

case "${1:-}" in
  --explain) show_explain; exit 0 ;;
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
      -- set-env "${UNIT}" "FEATURE_INVENTORY_RESERVATIONMODE=strict" 2>/dev/null || true
    sleep 2
    echo "Done."
    echo ""
    echo "Verify: ./compare.sh"
    exit 0
    ;;
  "") ;;
  *) echo "Usage: $0 [--explain|--dry-run|--revert]" >&2; exit 2 ;;
esac

command -v "${CUB}" >/dev/null 2>&1 || { echo "error: cub CLI not found." >&2; exit 1; }

# Decode unit data from cub JSON response (base64 Data → YAML docs array)
decode_unit_data() {
  local raw="$1"
  if echo "$raw" | jq -e 'type == "array"' >/dev/null 2>&1; then
    echo "$raw"
    return
  fi
  local b64
  b64=$(echo "$raw" | jq -r '.Unit.Data // empty' 2>/dev/null)
  if [[ -z "$b64" ]]; then
    echo "[]"
    return
  fi
  echo "$b64" | base64 -d 2>/dev/null | python3 -c "
import sys, json, yaml
docs = list(yaml.safe_load_all(sys.stdin))
json.dump([d for d in docs if d], sys.stdout)
" 2>/dev/null || echo "[]"
}

# Extract FEATURE_INVENTORY_RESERVATIONMODE from unit data
extract_reservation_mode() {
  local raw="$1" default_msg="$2"
  local data
  data=$(decode_unit_data "$raw")
  local val
  val=$(echo "$data" | jq -r '[.[] | select(.kind == "Deployment")
    | .spec.template.spec.containers[]?.env[]?
    | select(.name == "FEATURE_INVENTORY_RESERVATIONMODE")
    | .value] | first // empty' 2>/dev/null)
  if [[ -n "$val" ]]; then
    echo "  FEATURE_INVENTORY_RESERVATIONMODE = ${val}"
  else
    echo "  ${default_msg}"
  fi
}

echo "=== Platform Write API — the mutation ==="
echo ""
echo "Changing feature.inventory.reservationMode for ${SPACE}:"
echo "  strict → optimistic"
echo ""

# Show before
echo "Before:"
before_raw=$(${CUB} unit get --space "${SPACE}" --data-only --json "${UNIT}" 2>/dev/null || true)
if [[ -n "$before_raw" && "$before_raw" != "null" ]]; then
  extract_reservation_mode "$before_raw" "(not set — using ConfigMap default: strict)"
else
  echo "  (could not read current value)"
fi

echo ""

# Mutate
echo "Mutating..."
${CUB} function do --space "${SPACE}" \
  --change-desc "apply-here: reservation mode rollout (strict → optimistic)" \
  -- set-env "${UNIT}" "FEATURE_INVENTORY_RESERVATIONMODE=optimistic" 2>/dev/null || true

# Wait briefly for the no-op worker to process
sleep 2

echo ""

# Show after
echo "After:"
after_raw=$(${CUB} unit get --space "${SPACE}" --data-only --json "${UNIT}" 2>/dev/null || true)
if [[ -n "$after_raw" && "$after_raw" != "null" ]]; then
  extract_reservation_mode "$after_raw" "(mutation queued — waiting for worker)"
else
  echo "  (could not read new value)"
fi

echo ""

# Show mutation history
echo "Mutation history (last 3):"
${CUB} mutation list --space "${SPACE}" --json "${UNIT}" 2>/dev/null | \
  jq -r '.[-3:][] | "  #\(.Mutation.MutationNum) \(.Revision.CreatedAt | split("T")[0]) \(.Revision.Description // "no description")"' 2>/dev/null || \
  echo "  (mutation list not available)"

echo ""
echo "That was the write API."
echo ""
echo "Next:"
echo "  ./compare.sh           — see the divergence marker on prod"
echo "  ./refresh-preview.sh   — see that this mutation would survive a refresh"
echo "  ./mutate.sh --revert   — revert to the upstream default"
