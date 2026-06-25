#!/usr/bin/env bash
# setup.sh — Install the cost-estimator budget guardrail Triggers for real use
#
# Installs the validation the estimator relies on. The guardrails are defined
# ONCE in a single policy Space and enforced across the fleet via a Trigger
# Filter — not copied into every Space:
#
#   valid-schemas      vet-schemas  Kubernetes schema validation
#   requests-required  vet-celexpr  every container must declare cpu+memory requests
#   within-budget      vet-celexpr  no workloads the estimator flagged OVER budget
#
# How it works:
#   1. The Triggers are created in a policy Space (default: policy-guardrails,
#      override with --policy-space), labeled Pack=cost-guardrails.
#   2. A Trigger Filter in that Space selects them: Labels.Pack = 'cost-guardrails'.
#   3. Each in-scope Space's TriggerFilterID is pointed at that Filter, so the
#      one definition is enforced everywhere. (A Space's WhereTrigger and
#      TriggerFilterID are ANDed; the default WhereTrigger restricts to the
#      Space's own Triggers, so it is cleared when wiring to the shared Filter.)
#
# Triggers are created with Warn=true: violations produce ApplyWarnings
# (advisory) rather than ApplyGates (blocking), so installing them on an
# existing fleet never blocks anyone. Promote one to blocking with:
#
#   cub trigger update <slug> --space <policy-space> --unwarn
#
# within-budget gates on a cost-estimator.confighub.com/budget-status annotation
# the estimator writes onto each Unit. Until you estimate (estimator/costest
# estimate-fleet --write-back), Units carry no such annotation and the guardrail
# passes them; it only warns/blocks once an OVER estimate has been recorded.
#
# Scope: by default, every Space you can view that contains Kubernetes/YAML
# Units. Narrow with a ConfigHub filter expression:
#
#   ./setup.sh --where-space "Slug LIKE 'prod-%'"
#   ./setup.sh --where-space "Labels.Environment = 'Prod'"
#
# This script does not seed any data. For the self-contained demo fleet,
# see demo-setup.sh.
#
# Prerequisites:
#   - cub CLI installed: https://docs.confighub.com/get-started/setup/#install-the-cli
#   - Authenticated: cub auth login
#
# Usage:
#   ./setup.sh [--policy-space SLUG] [--where-space EXPR]   # install (idempotent)
#   ./setup.sh --explain                                    # print the plan, mutate nothing
#   ./setup.sh --explain-json                               # print the plan as JSON
#
# Environment variables:
#   CUB      Path to cub binary (default: cub on PATH)

set -euo pipefail

cub="${CUB:-cub}"
EXAMPLE_NAME="cost-estimator"
POLICY_SPACE="policy-guardrails"
FILTER_SLUG="cost-guardrails"
WHERE_SPACE=""

# Guardrail CEL expressions (validated against the manifests in this example
# with `cub function local <manifest> vet-celexpr '<expr>' --toolchain Kubernetes/YAML`).
REQUESTS_REQUIRED="!(r.kind in ['Deployment','StatefulSet']) || r.spec.template.spec.containers.all(c, has(c.resources) && has(c.resources.requests) && 'cpu' in c.resources.requests && 'memory' in c.resources.requests)"
WITHIN_BUDGET="!(r.kind in ['Deployment','StatefulSet']) || !has(r.metadata.annotations) || !('cost-estimator.confighub.com/budget-status' in r.metadata.annotations) || r.metadata.annotations['cost-estimator.confighub.com/budget-status'] != 'OVER'"

# ── Argument parsing ──────────────────────────────────────────────────────────

MODE="run"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain) MODE="explain"; shift ;;
    --explain-json) MODE="explain-json"; shift ;;
    --policy-space) POLICY_SPACE="${2:?--policy-space requires a Space slug}"; shift 2 ;;
    --where-space) WHERE_SPACE="${2:?--where-space requires an expression}"; shift 2 ;;
    *) echo "Unknown argument: $1 (supported: --policy-space SLUG, --where-space EXPR, --explain, --explain-json)" >&2; exit 2 ;;
  esac
done

FILTER_REF="${POLICY_SPACE}/${FILTER_SLUG}"

# ── Explain modes (no mutation) ───────────────────────────────────────────────

explain() {
  cat <<EOF
cost-estimator guardrail setup plan
===================================

Defines the budget guardrail Triggers (Warn=true → ApplyWarnings, never blocking)
ONCE in the policy Space "${POLICY_SPACE}", and enforces them fleet-wide via a
shared Trigger Filter ("${FILTER_SLUG}"):

  valid-schemas       Kubernetes schema validation (kubeconform)
  requests-required   blocks workloads with no cpu/memory requests (uncostable)
  within-budget       blocks workloads the estimator flagged OVER budget (annotation-driven)

Each in-scope Space's TriggerFilterID is pointed at "${FILTER_REF}".

Scope: Spaces containing Kubernetes/YAML Units${WHERE_SPACE:+ matching: ${WHERE_SPACE}}
(default: everything you have permission to view).

Spaces with a custom WhereTrigger, an existing TriggerFilterID, or Triggers of
their own are reported, not modified. Idempotent. Promote a guardrail to
blocking later with: cub trigger update <slug> --space ${POLICY_SPACE} --unwarn
(one change enforces it across the whole fleet).

Then estimate your fleet to populate the budget verdicts the within-budget
guardrail reads: estimator/costest estimate-fleet --space "<your-spaces>" --write-back
EOF
}

explain_json() {
  cat <<EOF
{
  "example_name": "${EXAMPLE_NAME}",
  "mutates": true,
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "spaces": ["${POLICY_SPACE}"],
  "units": [],
  "notes": {
    "creates": "3 Warn=true guardrail Triggers + 1 Trigger Filter in ${POLICY_SPACE} (label Pack=cost-guardrails)",
    "wires": "Points each in-scope Space's TriggerFilterID at ${FILTER_REF}",
    "scope": "Spaces with Kubernetes/YAML Units${WHERE_SPACE:+, narrowed by --where-space}",
    "skips": "Spaces with a custom WhereTrigger, an existing TriggerFilterID, or their own Triggers (reported instead)",
    "depends_on_estimate": "within-budget reads a cost-estimator.confighub.com/budget-status annotation written by estimator/costest estimate-fleet --write-back"
  },
  "evaluation_modes": {
    "fast_preview": {
      "mutates": false,
      "commands": ["./setup.sh --explain", "./setup.sh --explain-json | jq"]
    },
    "fast_operational_evaluation": {
      "mutates_confighub": true,
      "mutates_live_infra": false,
      "commands": ["./setup.sh", "./verify.sh"],
      "stop_before_cleanup": true
    }
  }
}
EOF
}

case "$MODE" in
  explain) explain; exit 0 ;;
  explain-json) explain_json; exit 0 ;;
esac

# ── Preflight ─────────────────────────────────────────────────────────────────

if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found. Install it from https://docs.confighub.com/get-started/setup/#install-the-cli" >&2
  exit 1
fi
if ! $cub space list --quiet &>/dev/null; then
  echo "ERROR: Cannot reach ConfigHub (not authenticated?). Run: $cub auth login" >&2
  exit 1
fi

# ── 1. Policy Space: the guardrail Triggers + selecting Filter ────────────────

created=0
skipped=0
reported=0
wired=0

if $cub space get "$POLICY_SPACE" --quiet &>/dev/null; then
  echo "Policy Space ${POLICY_SPACE} exists."
else
  $cub space create "$POLICY_SPACE" --label app=cost-estimator --label role=policy >/dev/null
  echo "Created policy Space ${POLICY_SPACE}."
  ((created+=1))
fi

create_trigger() { # slug description function-args...
  local slug="$1" desc="$2"; shift 2
  if $cub trigger get "$slug" --space "$POLICY_SPACE" --quiet &>/dev/null; then
    ((skipped+=1))
  else
    $cub trigger create --space "$POLICY_SPACE" --warn \
      --label Pack=cost-guardrails \
      --description "$desc" \
      "$slug" Mutation Kubernetes/YAML "$@" >/dev/null
    echo "  created trigger ${POLICY_SPACE}/${slug} (warn)"
    ((created+=1))
  fi
}

create_trigger valid-schemas \
  "Validates Kubernetes resource schemas with kubeconform. Fix: correct the field names/types reported." \
  vet-schemas
create_trigger requests-required \
  "Warns on workloads whose containers omit cpu/memory requests (uncostable + a scheduling hazard). Fix: add resources.requests.cpu and .memory." \
  vet-celexpr "$REQUESTS_REQUIRED"
create_trigger within-budget \
  "Warns on workloads the estimator flagged OVER their environment budget (cost-estimator.confighub.com/budget-status). Fix: cut replicas/requests, or raise the budget, then re-estimate." \
  vet-celexpr "$WITHIN_BUDGET"

# Trigger Filter that selects the guardrail pack (label-based, so it keeps
# working if more guardrails are added to the pack later).
if $cub filter get "$FILTER_SLUG" --space "$POLICY_SPACE" --quiet &>/dev/null; then
  ((skipped+=1))
else
  $cub filter create --space "$POLICY_SPACE" "$FILTER_SLUG" Trigger \
    --where-field "Labels.Pack = 'cost-guardrails'" >/dev/null
  echo "  created filter ${FILTER_REF}"
  ((created+=1))
fi
FILTER_ID=$($cub filter get "$FILTER_SLUG" --space "$POLICY_SPACE" -o jq='.Filter.FilterID' 2>/dev/null | tr -d '"')
echo

# ── 2. Resolve scope: spaces with Kubernetes/YAML units, optionally narrowed ──

if [[ -n "$WHERE_SPACE" ]]; then
  selected_spaces=$($cub space list --where "$WHERE_SPACE" -o jq='.[].Space.Slug' 2>/dev/null | tr -d '"')
else
  selected_spaces=$($cub space list -o jq='.[].Space.Slug' 2>/dev/null | tr -d '"')
fi

k8s_spaces=$($cub unit list --space "*" --where "ToolchainType = 'Kubernetes/YAML'" \
  -o jq='.[].Space.Slug' 2>/dev/null | tr -d '"' | sort -u)

scope=$(comm -12 <(echo "$selected_spaces" | sort -u) <(echo "$k8s_spaces") | grep -vx "$POLICY_SPACE" || true)
if [[ -z "$scope" ]]; then
  echo "No in-scope Spaces with Kubernetes/YAML Units found (outside the policy Space)." >&2
  exit 1
fi

echo "In-scope Spaces:"
echo "$scope" | sed 's/^/  /'
echo

# ── 3. Point each in-scope Space's TriggerFilterID at the shared Filter ───────

for space in $scope; do
  config=$($cub space get "$space" -o jq='{w: .Space.WhereTrigger, f: .Space.TriggerFilterID, id: .Space.SpaceID}' 2>/dev/null)
  where_trigger=$(echo "$config" | /usr/bin/python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('w') or '')")
  trigger_filter=$(echo "$config" | /usr/bin/python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('f') or '')")
  space_id=$(echo "$config" | /usr/bin/python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('id') or '')")

  if [[ -n "$trigger_filter" ]]; then
    if [[ "$trigger_filter" == "$FILTER_ID" ]]; then
      ((skipped+=1)); continue
    fi
    echo "  SKIP ${space}: already has a different TriggerFilterID — add the '${FILTER_SLUG}' guardrails to that Filter's set"
    ((reported+=1)); continue
  fi

  if [[ -n "$where_trigger" && "$where_trigger" != *"$space_id"* ]]; then
    echo "  SKIP ${space}: custom WhereTrigger (${where_trigger}) — point it at ${FILTER_REF} as well"
    ((reported+=1)); continue
  fi

  own_count=$($cub trigger list --space "$space" -o jq='.[].Trigger.Slug' 2>/dev/null | grep -c . || true)
  if [[ "${own_count:-0}" -gt 0 ]]; then
    echo "  SKIP ${space}: has ${own_count} Trigger(s) of its own — add the guardrail Filter to its WhereTrigger to keep both"
    ((reported+=1)); continue
  fi

  $cub space update "$space" --where-trigger - --trigger-filter "$FILTER_REF" >/dev/null
  echo "  wired ${space} → ${FILTER_REF}"
  ((wired+=1))
done

cat <<EOF

Done. Created ${created} object(s), wired ${wired} Space(s), skipped ${skipped} already set, reported ${reported} Space(s) with custom trigger wiring.

Violations now surface as ApplyWarnings (advisory). Review them with:
  $cub unit list --space "*" --where "LEN(ApplyWarnings) > 0"

Estimate your fleet so within-budget has cost verdicts to act on:
  ./estimator/costest estimate-fleet --space "<your-spaces>" --write-back

Promote a guardrail to blocking (ApplyGates) — one change, fleet-wide:
  $cub trigger update within-budget --space ${POLICY_SPACE} --unwarn
EOF
