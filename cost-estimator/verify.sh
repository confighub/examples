#!/usr/bin/env bash
# verify.sh — Confirm the cost-estimator guardrails installed by setup.sh
#
# Read-only. Checks that:
#   1. The policy Space holds the three budget guardrail Triggers (Warn mode)
#      and the selecting Filter.
#   2. Every in-scope Space (Kubernetes/YAML Units, optionally narrowed with
#      --where-space) points its TriggerFilterID at that Filter.
#
# Spaces with a custom WhereTrigger, a different TriggerFilterID, or Triggers
# of their own are reported as out-of-scope passes (setup.sh leaves them
# alone). For the demo-fleet verification (gate matrix etc.), see
# demo-verify.sh.
#
# Usage:
#   ./verify.sh [--policy-space SLUG] [--where-space EXPR]
#
# Stable success text: "All checks passed."

set -euo pipefail

cub="${CUB:-cub}"
POLICY_SPACE="policy-guardrails"
FILTER_SLUG="cost-guardrails"
WHERE_SPACE=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --policy-space) POLICY_SPACE="${2:?--policy-space requires a Space slug}"; shift 2 ;;
    --where-space) WHERE_SPACE="${2:?--where-space requires an expression}"; shift 2 ;;
    *) echo "Unknown argument: $1" >&2; exit 2 ;;
  esac
done

if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found on PATH (set CUB=/path/to/cub)" >&2
  exit 1
fi

failures=0
checks=0
pass() { checks=$((checks + 1)); printf 'ok   %s\n' "$1"; }
fail() { checks=$((checks + 1)); failures=$((failures + 1)); printf 'FAIL %s\n' "$1"; }

# ── 1. Policy Space: Triggers + Filter ────────────────────────────────────────

for trigger in valid-schemas requests-required within-budget; do
  warn=$($cub trigger get "$trigger" --space "$POLICY_SPACE" -o jq='.Trigger.Warn' 2>/dev/null | tr -d '"' || echo missing)
  case "$warn" in
    true) pass "trigger ${POLICY_SPACE}/${trigger} exists (warn)" ;;
    false) pass "trigger ${POLICY_SPACE}/${trigger} exists (promoted to blocking)" ;;
    *) fail "trigger ${POLICY_SPACE}/${trigger} exists" ;;
  esac
done

FILTER_ID=$($cub filter get "$FILTER_SLUG" --space "$POLICY_SPACE" -o jq='.Filter.FilterID' 2>/dev/null | tr -d '"' || true)
if [[ -n "$FILTER_ID" ]]; then
  pass "filter ${POLICY_SPACE}/${FILTER_SLUG} exists"
else
  fail "filter ${POLICY_SPACE}/${FILTER_SLUG} exists"
fi

# ── 2. In-scope Spaces point at the Filter ────────────────────────────────────

if [[ -n "$WHERE_SPACE" ]]; then
  selected_spaces=$($cub space list --where "$WHERE_SPACE" -o jq='.[].Space.Slug' 2>/dev/null | tr -d '"')
else
  selected_spaces=$($cub space list -o jq='.[].Space.Slug' 2>/dev/null | tr -d '"')
fi
k8s_spaces=$($cub unit list --space "*" --where "ToolchainType = 'Kubernetes/YAML'" \
  -o jq='.[].Space.Slug' 2>/dev/null | tr -d '"' | sort -u)
scope=$(comm -12 <(echo "$selected_spaces" | sort -u) <(echo "$k8s_spaces") | grep -vx "$POLICY_SPACE" || true)

for space in $scope; do
  config=$($cub space get "$space" -o jq='{w: .Space.WhereTrigger, f: .Space.TriggerFilterID, id: .Space.SpaceID}' 2>/dev/null)
  where_trigger=$(echo "$config" | /usr/bin/python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('w') or '')")
  trigger_filter=$(echo "$config" | /usr/bin/python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('f') or '')")
  space_id=$(echo "$config" | /usr/bin/python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('id') or '')")
  own_count=$($cub trigger list --space "$space" -o jq='.[].Trigger.Slug' 2>/dev/null | grep -c . || true)

  if [[ "$trigger_filter" == "$FILTER_ID" && -n "$FILTER_ID" ]]; then
    pass "space ${space}: wired to ${POLICY_SPACE}/${FILTER_SLUG}"
  elif [[ -n "$trigger_filter" || ( -n "$where_trigger" && "$where_trigger" != *"$space_id"* ) || "${own_count:-0}" -gt 0 ]]; then
    pass "space ${space}: custom trigger wiring (out of setup.sh scope)"
  else
    fail "space ${space}: wired to ${POLICY_SPACE}/${FILTER_SLUG}"
  fi
done

echo
if (( failures > 0 )); then
  echo "${failures} of ${checks} checks FAILED."
  exit 1
fi
echo "All checks passed. (${checks} checks)"
