#!/usr/bin/env bash
# demo-verify.sh — Confirm the sec-scanner demo fleet seeded by demo-setup.sh
#
# Read-only on ConfigHub. Asserts the Space/Trigger/Filter/Unit layout, the
# gate matrix (each planted violation carries exactly its intended Apply Gate,
# clean workloads are ungated, prod requires approval), that the scanner wrote
# its findings back as data, and that the cvedb holds advisories.
#
# Usage:   ./demo-verify.sh
#
# Environment variables:
#   PREFIX           Space slug prefix (default: sec-demo) — must match setup
#   CUB              Path to cub binary (default: cub on PATH)
#   SEC_SCANNER_DB   cvedb SQLite path for the cvedb check (default: cvedb/cve.db)
#
# Stable success text: "All checks passed."

set -euo pipefail

PREFIX="${PREFIX:-sec-demo}"
cub="${CUB:-cub}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
export SEC_SCANNER_DB="${SEC_SCANNER_DB:-${SCRIPT_DIR}/cvedb/cve.db}"

POLICY_SPACE="${PREFIX}-policy"
BASE_SPACE="${PREFIX}-base"
DEV_SPACE="${PREFIX}-dev"
STAGING_SPACE="${PREFIX}-staging"
PROD_SPACE="${PREFIX}-prod"
WORKLOADS=(frontend api cache)
TRIGGERS=(valid-schemas no-latest-tag no-critical-cves require-approval)

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
for filter in sec-guardrails sec-guardrails-prod; do
  check "filter ${POLICY_SPACE}/${filter} exists" $cub filter get "$filter" --space "$POLICY_SPACE" --quiet
done
for space in "$BASE_SPACE" "$DEV_SPACE" "$STAGING_SPACE" "$PROD_SPACE"; do
  for w in "${WORKLOADS[@]}"; do
    check "unit ${space}/${w} exists" $cub unit get "$w" --space "$space" --quiet
  done
done
for v in legacy-frontend legacy-api unpinned-web; do
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

# ── Scan write-back ─────────────────────────────────────────────────────────────
# The scanner annotated each Unit with its max CVE severity. The vulnerable
# planted Units must be marked CRITICAL.
data_severity() { $cub unit data --space "$1" "$2" 2>/dev/null | grep -A2 'max-severity' | grep -oE 'CRITICAL|HIGH|MEDIUM|LOW|NONE' | head -1; }
for v in legacy-frontend legacy-api; do
  if [[ "$(data_severity "$DEV_SPACE" "$v")" == "CRITICAL" ]]; then
    pass "scan: ${DEV_SPACE}/${v} annotated max-severity=CRITICAL"
  else
    fail "scan: ${DEV_SPACE}/${v} annotated max-severity=CRITICAL (run scan-fleet --write-back)"
  fi
done

# The full findings live in one AppConfig/YAML "sec-scan-record" Unit per Space:
# a multi-document YAML, one document (keyed by unit:) per scanned workload.
check "scan-record ${DEV_SPACE}/sec-scan-record exists" $cub unit get sec-scan-record --space "$DEV_SPACE" --quiet
record_data="$($cub unit data --space "$DEV_SPACE" sec-scan-record 2>/dev/null || true)"
if grep -q 'cve_count:' <<<"$record_data"; then
  pass "scan-record sec-scan-record holds findings (cve_count)"
else
  fail "scan-record sec-scan-record holds findings"
fi
# It is a multi-doc YAML carrying both planted violations as separate documents.
if grep -q '^unit: legacy-frontend$' <<<"$record_data" && grep -q '^unit: legacy-api$' <<<"$record_data"; then
  pass "scan-record holds per-workload documents (legacy-frontend, legacy-api)"
else
  fail "scan-record holds per-workload documents (legacy-frontend, legacy-api)"
fi

# Scan provenance: units record the CVE DB version they were scanned against,
# and the policy Space holds the current cvedb-status.
if $cub unit data --space "$DEV_SPACE" legacy-frontend 2>/dev/null | grep -q 'cvedb-version'; then
  pass "scan: ${DEV_SPACE}/legacy-frontend records cvedb-version"
else
  fail "scan: ${DEV_SPACE}/legacy-frontend records cvedb-version"
fi
if $cub unit data --space "$POLICY_SPACE" cvedb-status 2>/dev/null | grep -q 'cvedb_version:'; then
  pass "cvedb-status Unit present in ${POLICY_SPACE}"
else
  fail "cvedb-status Unit present in ${POLICY_SPACE} (run scan-fleet --status-space ${POLICY_SPACE})"
fi

# ── Gate matrix ───────────────────────────────────────────────────────────────

for unit in legacy-frontend legacy-api unpinned-web; do
  wait_for_triggers "$DEV_SPACE" "$unit" || fail "trigger evaluation settled for ${DEV_SPACE}/${unit}"
done
wait_for_triggers "$PROD_SPACE" frontend || fail "trigger evaluation settled for ${PROD_SPACE}/frontend"

check "gate: legacy-frontend blocked by no-critical-cves" \
  has_gate "$DEV_SPACE" legacy-frontend "${POLICY_SPACE}/no-critical-cves/vet-celexpr"
check "gate: legacy-api blocked by no-critical-cves" \
  has_gate "$DEV_SPACE" legacy-api "${POLICY_SPACE}/no-critical-cves/vet-celexpr"
check "gate: unpinned-web blocked by no-latest-tag" \
  has_gate "$DEV_SPACE" unpinned-web "${POLICY_SPACE}/no-latest-tag/vet-celexpr"
check "gate: ${PROD_SPACE}/frontend requires approval" \
  has_gate "$PROD_SPACE" frontend "${POLICY_SPACE}/require-approval/vet-approvedby"
check "no gate: ${STAGING_SPACE}/frontend (current image passes the pack)" \
  no_gates "$STAGING_SPACE" frontend
check "no gate: ${DEV_SPACE}/frontend (current image passes the pack)" \
  no_gates "$DEV_SPACE" frontend

# ── cvedb ───────────────────────────────────────────────────────────────────────

if command -v sqlite3 &>/dev/null; then
  adv="$(sqlite3 "$SEC_SCANNER_DB" 'SELECT count(*) FROM advisory' 2>/dev/null || echo 0)"
  if [[ "${adv:-0}" -gt 0 ]]; then
    pass "cvedb holds ${adv} advisories"
  else
    fail "cvedb holds advisories (got ${adv:-0}; run ./cvedb/build.sh)"
  fi
else
  pass "cvedb check skipped (sqlite3 not on PATH)"
fi

# ── Result ────────────────────────────────────────────────────────────────────

echo
if (( failures > 0 )); then
  echo "${failures} of ${checks} checks FAILED."
  exit 1
fi
echo "All checks passed. (${checks} checks)"
