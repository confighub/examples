#!/usr/bin/env bash
# demo-verify.sh — Confirm the cost-estimator demo fleet seeded by demo-setup.sh
#
# Read-only on ConfigHub. Asserts the Space/Trigger/Filter/Unit layout, the
# gate matrix (each planted violation carries exactly its intended Apply Gate,
# clean workloads are ungated, prod requires approval), that the estimator wrote
# its estimates back as data, and that the price book is present.
#
# Usage:   ./demo-verify.sh
#
# Environment variables:
#   PREFIX   Space slug prefix (default: cost-demo) — must match setup
#   CUB      Path to cub binary (default: cub on PATH)
#
# Stable success text: "All checks passed."

set -euo pipefail

PREFIX="${PREFIX:-cost-demo}"
cub="${CUB:-cub}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

POLICY_SPACE="${PREFIX}-policy"
BASE_SPACE="${PREFIX}-base"
DEV_SPACE="${PREFIX}-dev"
STAGING_SPACE="${PREFIX}-staging"
PROD_SPACE="${PREFIX}-prod"
WORKLOADS=(frontend api cache db)
TRIGGERS=(valid-schemas requests-required within-budget require-approval)

command -v "$cub" &>/dev/null || { echo "ERROR: cub not found on PATH (set CUB=/path/to/cub)" >&2; exit 1; }

failures=0
checks=0
pass() { checks=$((checks + 1)); printf 'ok   %s\n' "$1"; }
fail() { checks=$((checks + 1)); failures=$((failures + 1)); printf 'FAIL %s\n' "$1"; }
check() { local desc="$1"; shift; if "$@" &>/dev/null; then pass "$desc"; else fail "$desc"; fi; }

gates_of()  { $cub unit get "$2" --space "$1" -o jq=".Unit.ApplyGates" 2>/dev/null; }
has_gate()  { gates_of "$1" "$2" | grep -q "$3"; }
no_gates()  { local g; g="$(gates_of "$1" "$2")"; [[ "$g" == "null" || "$g" == "{}" ]]; }

# Trigger evaluation is asynchronous; let a freshly-mutated Unit settle.
wait_for_triggers() {
  local i
  for i in $(seq 1 30); do
    gates_of "$1" "$2" | grep -q 'awaiting/triggers' || return 0
    sleep 2
  done
  return 1
}

# ── Layout ────────────────────────────────────────────────────────────────────

for space in "$POLICY_SPACE" "$BASE_SPACE" "$DEV_SPACE" "$STAGING_SPACE" "$PROD_SPACE"; do
  check "space ${space} exists" $cub space get "$space" --quiet
done
for trigger in "${TRIGGERS[@]}"; do
  check "trigger ${POLICY_SPACE}/${trigger} exists" $cub trigger get "$trigger" --space "$POLICY_SPACE" --quiet
done
for filter in cost-guardrails cost-guardrails-prod; do
  check "filter ${POLICY_SPACE}/${filter} exists" $cub filter get "$filter" --space "$POLICY_SPACE" --quiet
done
for space in "$BASE_SPACE" "$DEV_SPACE" "$STAGING_SPACE" "$PROD_SPACE"; do
  for w in "${WORKLOADS[@]}"; do
    check "unit ${space}/${w} exists" $cub unit get "$w" --space "$space" --quiet
  done
done
for v in oversized-analytics no-requests-web; do
  check "unit ${DEV_SPACE}/${v} exists" $cub unit get "$v" --space "$DEV_SPACE" --quiet
done

# Cluster Spaces actually selected the guardrail Triggers (TriggerFilterID):
# dev/staging select the 3 Scope=all triggers; prod selects all 4.
trigger_count() { $cub space get "$1" -o jq=".Space.TriggerIDs | length" 2>/dev/null; }
for space in "$DEV_SPACE" "$STAGING_SPACE"; do
  if [[ "$(trigger_count "$space")" == "3" ]]; then
    pass "space ${space} selects 3 guardrail triggers"
  else
    fail "space ${space} selects 3 guardrail triggers (got $(trigger_count "$space"))"
  fi
done
if [[ "$(trigger_count "$PROD_SPACE")" == "4" ]]; then
  pass "space ${PROD_SPACE} selects 4 guardrail triggers (incl. approval)"
else
  fail "space ${PROD_SPACE} selects 4 guardrail triggers (got $(trigger_count "$PROD_SPACE"))"
fi

# ── Estimate write-back ──────────────────────────────────────────────────────
# The estimator annotated each workload with its budget verdict. The
# over-provisioned planted Unit must be marked OVER.
budget_status() { $cub unit data --space "$1" "$2" 2>/dev/null | grep 'budget-status' | grep -oE 'OVER|WARN|UNKNOWN|OK' | head -1; }
if [[ "$(budget_status "$DEV_SPACE" oversized-analytics)" == "OVER" ]]; then
  pass "estimate: ${DEV_SPACE}/oversized-analytics annotated budget-status=OVER"
else
  fail "estimate: ${DEV_SPACE}/oversized-analytics annotated budget-status=OVER (run estimate-fleet --write-back)"
fi
if $cub unit data --space "$DEV_SPACE" frontend 2>/dev/null | grep -q 'monthly-usd'; then
  pass "estimate: ${DEV_SPACE}/frontend annotated monthly-usd"
else
  fail "estimate: ${DEV_SPACE}/frontend annotated monthly-usd"
fi

# The full estimates live in one AppConfig/YAML "cost-estimate-record" Unit per
# Space (a list of per-workload reports).
check "cost-record ${DEV_SPACE}/cost-estimate-record exists" $cub unit get cost-estimate-record --space "$DEV_SPACE" --quiet
record_data="$($cub unit data --space "$DEV_SPACE" cost-estimate-record 2>/dev/null || true)"
if grep -q 'monthly_usd:' <<<"$record_data" && grep -q 'oversized-analytics' <<<"$record_data"; then
  pass "cost-estimate-record holds per-workload estimates (monthly_usd, oversized-analytics)"
else
  fail "cost-estimate-record holds per-workload estimates"
fi

# Provenance: units record the pricing version, and the policy Space holds the
# current pricebook-status.
if $cub unit data --space "$DEV_SPACE" frontend 2>/dev/null | grep -q 'pricing-version'; then
  pass "estimate: ${DEV_SPACE}/frontend records pricing-version"
else
  fail "estimate: ${DEV_SPACE}/frontend records pricing-version"
fi
if $cub unit data --space "$POLICY_SPACE" pricebook-status 2>/dev/null | grep -q 'pricingVersion:'; then
  pass "pricebook-status Unit present in ${POLICY_SPACE}"
else
  fail "pricebook-status Unit present in ${POLICY_SPACE} (run estimate-fleet --status-space ${POLICY_SPACE})"
fi

# ── Gate matrix ───────────────────────────────────────────────────────────────

for unit in oversized-analytics no-requests-web; do
  wait_for_triggers "$DEV_SPACE" "$unit" || fail "trigger evaluation settled for ${DEV_SPACE}/${unit}"
done
wait_for_triggers "$PROD_SPACE" frontend || fail "trigger evaluation settled for ${PROD_SPACE}/frontend"

check "gate: oversized-analytics blocked by within-budget" \
  has_gate "$DEV_SPACE" oversized-analytics "${POLICY_SPACE}/within-budget/vet-celexpr"
check "gate: no-requests-web blocked by requests-required" \
  has_gate "$DEV_SPACE" no-requests-web "${POLICY_SPACE}/requests-required/vet-celexpr"
check "gate: ${PROD_SPACE}/frontend requires approval" \
  has_gate "$PROD_SPACE" frontend "${POLICY_SPACE}/require-approval/vet-approvedby"
check "no gate: ${STAGING_SPACE}/frontend (within budget, passes the pack)" \
  no_gates "$STAGING_SPACE" frontend
check "no gate: ${DEV_SPACE}/frontend (within budget, passes the pack)" \
  no_gates "$DEV_SPACE" frontend

# ── Price book ────────────────────────────────────────────────────────────────

if [[ -f "${SCRIPT_DIR}/pricing/pricebook.json" ]] && grep -q '"version"' "${SCRIPT_DIR}/pricing/pricebook.json"; then
  pass "price book present (pricing/pricebook.json)"
else
  fail "price book present (pricing/pricebook.json)"
fi

# ── Result ────────────────────────────────────────────────────────────────────

echo
if (( failures > 0 )); then
  echo "${failures} of ${checks} checks FAILED."
  exit 1
fi
echo "All checks passed. (${checks} checks)"
