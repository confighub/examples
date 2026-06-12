#!/usr/bin/env bash
# verify.sh — Confirm the rbac-manager demo fleet seeded by setup.sh
#
# Read-only: no ConfigHub or live-infrastructure mutation. Asserts the
# Space/Trigger/Filter/Unit layout and, critically, the gate matrix:
# each planted violation carries exactly its intended Apply Gate, the
# orphaned binding carries none (it is an app-side audit finding), prod
# requires approval, and clean personas are ungated.
#
# Usage:
#   ./verify.sh
#
# Environment variables:
#   PREFIX   Space slug prefix (default: rbac-demo) — must match setup.sh
#   CUB      Path to cub binary (default: cub on PATH)
#
# Stable success text: "All checks passed."

set -euo pipefail

PREFIX="${PREFIX:-rbac-demo}"
cub="${CUB:-cub}"

POLICY_SPACE="${PREFIX}-policy"
BASE_SPACE="${PREFIX}-base"
DEV_SPACE="${PREFIX}-dev"
STAGING_SPACE="${PREFIX}-staging"
PROD_SPACE="${PREFIX}-prod"
PERSONAS=(developer operator viewer ci)
TRIGGERS=(valid-rbac-schemas no-wildcards no-privilege-escalation no-cluster-admin-binding require-approval)

if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found on PATH (set CUB=/path/to/cub)" >&2
  exit 1
fi

failures=0
checks=0

pass() { checks=$((checks + 1)); printf 'ok   %s\n' "$1"; }
fail() { checks=$((checks + 1)); failures=$((failures + 1)); printf 'FAIL %s\n' "$1"; }

check() { # description, command...
  local desc="$1"; shift
  if "$@" &>/dev/null; then pass "$desc"; else fail "$desc"; fi
}

gates_of() { # space, unit → ApplyGates JSON (or "null")
  $cub unit get "$2" --space "$1" -o jq=".Unit.ApplyGates" 2>/dev/null
}

# Trigger evaluation is asynchronous: a fresh mutation briefly carries the
# awaiting/triggers gate. Wait for the dev violations to settle before
# asserting the matrix.
wait_for_triggers() { # space, unit
  local i
  for i in $(seq 1 30); do
    if ! gates_of "$1" "$2" | grep -q 'awaiting/triggers'; then
      return 0
    fi
    sleep 2
  done
  return 1
}

has_gate() { # space, unit, gate-substring
  gates_of "$1" "$2" | grep -q "$3"
}

no_gates() { # space, unit
  local g
  g="$(gates_of "$1" "$2")"
  [[ "$g" == "null" || "$g" == "{}" ]]
}

# ── Layout ────────────────────────────────────────────────────────────────────

for space in "$POLICY_SPACE" "$BASE_SPACE" "$DEV_SPACE" "$STAGING_SPACE" "$PROD_SPACE"; do
  check "space ${space} exists" $cub space get "$space" --quiet
done

for trigger in "${TRIGGERS[@]}"; do
  check "trigger ${POLICY_SPACE}/${trigger} exists" $cub trigger get "$trigger" --space "$POLICY_SPACE" --quiet
done

for filter in rbac-guardrails rbac-guardrails-prod; do
  check "filter ${POLICY_SPACE}/${filter} exists" $cub filter get "$filter" --space "$POLICY_SPACE" --quiet
done

for space in "$BASE_SPACE" "$DEV_SPACE" "$STAGING_SPACE" "$PROD_SPACE"; do
  for persona in "${PERSONAS[@]}"; do
    check "unit ${space}/${persona} exists" $cub unit get "$persona" --space "$space" --quiet
  done
done

for violation in legacy-wildcard-admin orphaned-grafana-binding breakglass-cluster-admin; do
  check "unit ${DEV_SPACE}/${violation} exists" $cub unit get "$violation" --space "$DEV_SPACE" --quiet
done

# Cluster Spaces actually selected the guardrail Triggers (TriggerFilterID
# wiring): dev/staging select the 4 Scope=all triggers, prod all 5.
trigger_count() { $cub space get "$1" -o jq=".Space.TriggerIDs | length" 2>/dev/null; }
for space in "$DEV_SPACE" "$STAGING_SPACE"; do
  if [[ "$(trigger_count "$space")" == "4" ]]; then
    pass "space ${space} selects 4 guardrail triggers"
  else
    fail "space ${space} selects 4 guardrail triggers (got $(trigger_count "$space"))"
  fi
done
if [[ "$(trigger_count "$PROD_SPACE")" == "5" ]]; then
  pass "space ${PROD_SPACE} selects 5 guardrail triggers (incl. approval)"
else
  fail "space ${PROD_SPACE} selects 5 guardrail triggers (got $(trigger_count "$PROD_SPACE"))"
fi

# ── Divergence ────────────────────────────────────────────────────────────────

# Dev developer gained the delete verb; staging tracks base and did not.
# Strip full-line comments: the persona header comment mentions "delete".
data_has_delete() { # space
  $cub unit data --space "$1" developer 2>/dev/null \
    | grep -v '^[[:space:]]*#' | grep -qw delete
}
check "divergence: ${DEV_SPACE}/developer has the delete verb" data_has_delete "$DEV_SPACE"
if data_has_delete "$STAGING_SPACE"; then
  fail "no divergence: ${STAGING_SPACE}/developer unchanged from base"
else
  pass "no divergence: ${STAGING_SPACE}/developer unchanged from base"
fi

# ── Gate matrix ───────────────────────────────────────────────────────────────

for unit in legacy-wildcard-admin orphaned-grafana-binding breakglass-cluster-admin; do
  if ! wait_for_triggers "$DEV_SPACE" "$unit"; then
    fail "trigger evaluation settled for ${DEV_SPACE}/${unit} (awaiting/triggers stuck)"
  fi
done
if ! wait_for_triggers "$PROD_SPACE" developer; then
  fail "trigger evaluation settled for ${PROD_SPACE}/developer (awaiting/triggers stuck)"
fi

check "gate: legacy-wildcard-admin blocked by no-wildcards" \
  has_gate "$DEV_SPACE" legacy-wildcard-admin "${POLICY_SPACE}/no-wildcards/vet-celexpr"
check "gate: breakglass-cluster-admin blocked by no-cluster-admin-binding" \
  has_gate "$DEV_SPACE" breakglass-cluster-admin "${POLICY_SPACE}/no-cluster-admin-binding/vet-celexpr"
check "no gate: orphaned-grafana-binding (app-side audit finding only)" \
  no_gates "$DEV_SPACE" orphaned-grafana-binding
check "gate: ${PROD_SPACE}/developer requires approval" \
  has_gate "$PROD_SPACE" developer "${POLICY_SPACE}/require-approval/vet-approvedby"
check "no gate: ${STAGING_SPACE}/developer (clean persona passes the pack)" \
  no_gates "$STAGING_SPACE" developer
check "no gate: ${DEV_SPACE}/developer (divergence passes the pack)" \
  no_gates "$DEV_SPACE" developer

# ── Result ────────────────────────────────────────────────────────────────────

echo
if (( failures > 0 )); then
  echo "${failures} of ${checks} checks FAILED."
  exit 1
fi
echo "All checks passed. (${checks} checks)"
