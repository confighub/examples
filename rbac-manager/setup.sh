#!/usr/bin/env bash
# setup.sh — Install the RBAC guardrail Triggers for real use
#
# Sets up the validation the RBAC manager relies on. The guardrails are
# defined ONCE in a single policy Space and enforced across the fleet via a
# Trigger Filter — not copied into every Space:
#
#   no-rbac-wildcards             vet-celexpr  no wildcard verbs/resources/apiGroups
#   no-rbac-privilege-escalation  vet-celexpr  no escalate/bind/impersonate verbs
#   no-cluster-admin-binding      vet-celexpr  no ClusterRoleBindings to cluster-admin
#
# How it works:
#   1. The three Triggers are created in a policy Space (default: policy-guardrails,
#      override with --policy-space), labeled Pack=rbac-guardrails.
#   2. A Trigger Filter in that Space selects them: Labels.Pack = 'rbac-guardrails'.
#   3. Each in-scope Space's TriggerFilterID is pointed at that Filter, so the
#      one definition is enforced everywhere. (A Space's WhereTrigger and
#      TriggerFilterID are ANDed; the default WhereTrigger restricts to the
#      Space's own Triggers, so it is cleared when wiring to the shared Filter.)
#
# Triggers are created with Warn=true: violations produce ApplyWarnings
# (advisory) rather than ApplyGates (blocking), so installing them on an
# existing fleet never blocks anyone. Once the warnings are reviewed and
# resolved, promote a Trigger to blocking with:
#
#   cub trigger update <slug> --space <policy-space> --unwarn
#
# Because every Space shares the one Filter, that single change enforces the
# guardrail across the whole fleet.
#
# Scope: by default, every Space you can view that contains Kubernetes/YAML
# Units. Narrow with a ConfigHub filter expression:
#
#   ./setup.sh --where-space "Slug LIKE 'prod-%'"
#   ./setup.sh --where-space "Labels.Environment = 'prod'"
#
# Spaces that already select their Triggers another way (a custom WhereTrigger
# or an existing TriggerFilterID), or that have Triggers of their own, are NOT
# modified — they are reported so you can add the guardrail Filter to whatever
# they already select. Targets with their own Trigger selection are reported
# the same way.
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
EXAMPLE_NAME="rbac-manager"
POLICY_SPACE="policy-guardrails"
FILTER_SLUG="rbac-guardrails"
WHERE_SPACE=""

# Guardrail CEL expressions (validated against the manifests in this example
# with `cub function local <manifest> vet-celexpr '<expr>'`).
NO_WILDCARDS="!(r.kind in ['Role', 'ClusterRole']) || !has(r.rules) || !r.rules.exists(rule, (has(rule.verbs) && rule.verbs.exists(v, v == '*')) || (has(rule.resources) && rule.resources.exists(x, x == '*')) || (has(rule.apiGroups) && rule.apiGroups.exists(g, g == '*')))"
NO_ESCALATION="!(r.kind in ['Role', 'ClusterRole']) || !has(r.rules) || !r.rules.exists(rule, has(rule.verbs) && rule.verbs.exists(v, v in ['escalate', 'bind', 'impersonate']))"
NO_CLUSTER_ADMIN="r.kind != 'ClusterRoleBinding' || r.roleRef.name != 'cluster-admin'"

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
rbac-manager guardrail setup plan
=================================

Defines three RBAC guardrail Triggers (Warn=true → ApplyWarnings, never
blocking) ONCE in the policy Space "${POLICY_SPACE}", and enforces them
fleet-wide via a shared Trigger Filter ("${FILTER_SLUG}"):

  no-rbac-wildcards              blocks wildcard verbs/resources/apiGroups
  no-rbac-privilege-escalation   blocks escalate/bind/impersonate grants
  no-cluster-admin-binding       blocks ClusterRoleBindings to cluster-admin

Each in-scope Space's TriggerFilterID is pointed at "${FILTER_REF}".

Scope: Spaces containing Kubernetes/YAML Units${WHERE_SPACE:+ matching: ${WHERE_SPACE}}
(default: everything you have permission to view).

Spaces with a custom WhereTrigger, an existing TriggerFilterID, or Triggers of
their own are reported, not modified. Idempotent. Promote a guardrail to
blocking later with: cub trigger update <slug> --space ${POLICY_SPACE} --unwarn
(one change enforces it across the whole fleet).
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
    "creates": "3 Warn=true guardrail Triggers + 1 Trigger Filter in ${POLICY_SPACE} (label Pack=rbac-guardrails); 4 parameterized set-yq edit Invocations in rbac-edits",
    "wires": "Points each in-scope Space's TriggerFilterID at ${FILTER_REF}",
    "scope": "Spaces with Kubernetes/YAML Units${WHERE_SPACE:+, narrowed by --where-space}",
    "skips": "Spaces with a custom WhereTrigger, an existing TriggerFilterID, or their own Triggers (reported instead)"
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
  $cub space create "$POLICY_SPACE" --label app=rbac-manager --label role=policy >/dev/null
  echo "Created policy Space ${POLICY_SPACE}."
  ((created+=1))
fi

create_trigger() { # slug description function-args...
  local slug="$1" desc="$2"; shift 2
  if $cub trigger get "$slug" --space "$POLICY_SPACE" --quiet &>/dev/null; then
    ((skipped+=1))
  else
    $cub trigger create --space "$POLICY_SPACE" --warn \
      --label Pack=rbac-guardrails \
      --description "$desc" \
      "$slug" Mutation Kubernetes/YAML "$@" >/dev/null
    echo "  created trigger ${POLICY_SPACE}/${slug} (warn)"
    ((created+=1))
  fi
}

create_trigger no-rbac-wildcards \
  "Warns on Roles/ClusterRoles with wildcard verbs, resources, or apiGroups. Fix: enumerate the specific verbs/resources the role needs." \
  vet-celexpr "$NO_WILDCARDS"
create_trigger no-rbac-privilege-escalation \
  "Warns on Roles/ClusterRoles granting escalate, bind, or impersonate. Fix: remove these verbs; they allow privilege escalation." \
  vet-celexpr "$NO_ESCALATION"
create_trigger no-cluster-admin-binding \
  "Warns on ClusterRoleBindings to cluster-admin. Fix: bind a scoped role instead." \
  vet-celexpr "$NO_CLUSTER_ADMIN"

# Trigger Filter that selects the guardrail pack (label-based, so it keeps
# working if more guardrails are added to the pack later).
if $cub filter get "$FILTER_SLUG" --space "$POLICY_SPACE" --quiet &>/dev/null; then
  ((skipped+=1))
else
  $cub filter create --space "$POLICY_SPACE" "$FILTER_SLUG" Trigger \
    --where-field "Labels.Pack = 'rbac-guardrails'" >/dev/null
  echo "  created filter ${FILTER_REF}"
  ((created+=1))
fi
FILTER_ID=$($cub filter get "$FILTER_SLUG" --space "$POLICY_SPACE" -o jq='.Filter.FilterID' 2>/dev/null | tr -d '"')
echo

# ── 1b. Edit Invocations: shared, parameterized set-yq edits ──────────────────
# Both this app and the agent CLI reference these by slug and supply only the
# variable values as parameters, instead of compiling yq client-side.
source "$(dirname "${BASH_SOURCE[0]}")/install-edit-invocations.sh"
install_edit_invocations
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

  # Already wired to our Filter? (idempotent)
  if [[ -n "$trigger_filter" ]]; then
    if [[ "$trigger_filter" == "$FILTER_ID" ]]; then
      ((skipped+=1)); continue
    fi
    echo "  SKIP ${space}: already has a different TriggerFilterID — add the '${FILTER_SLUG}' guardrails to that Filter's set"
    ((reported+=1)); continue
  fi

  # Custom WhereTrigger (anything other than the default SpaceID=self)?
  if [[ -n "$where_trigger" && "$where_trigger" != *"$space_id"* ]]; then
    echo "  SKIP ${space}: custom WhereTrigger (${where_trigger}) — point it at ${FILTER_REF} as well"
    ((reported+=1)); continue
  fi

  # Own in-space Triggers? Wiring to the shared Filter would stop selecting them.
  own_count=$($cub trigger list --space "$space" -o jq='.[].Trigger.Slug' 2>/dev/null | grep -c . || true)
  if [[ "${own_count:-0}" -gt 0 ]]; then
    echo "  SKIP ${space}: has ${own_count} Trigger(s) of its own — add the guardrail Filter to its WhereTrigger to keep both"
    ((reported+=1)); continue
  fi

  # Clear the default WhereTrigger (SpaceID=self) and point at the shared Filter.
  $cub space update "$space" --where-trigger - --trigger-filter "$FILTER_REF" >/dev/null
  echo "  wired ${space} → ${FILTER_REF}"
  ((wired+=1))
done

# Targets with their own trigger selection get the same report-only treatment.
targets_with_config=$($cub target list --space "*" \
  -o jq='.[] | select((.Target.TriggerFilterID != null) or ((.Target.WhereTrigger // "") != "")) | "\(.Space.Slug)/\(.Target.Slug)"' 2>/dev/null | tr -d '"' || true)
if [[ -n "$targets_with_config" ]]; then
  echo
  echo "Targets with their own Trigger selection (point them at ${FILTER_REF} if desired):"
  echo "$targets_with_config" | sed 's/^/  /'
fi

cat <<EOF

Done. Created ${created} object(s), wired ${wired} Space(s), skipped ${skipped} already set, reported ${reported} Space(s) with custom trigger wiring.

Violations now surface as ApplyWarnings (advisory). Review them in the RBAC
manager app or with:
  $cub unit list --space "*" --where "LEN(ApplyWarnings) > 0"

Promote a guardrail to blocking (ApplyGates) — one change, fleet-wide:
  $cub trigger update no-rbac-wildcards --space ${POLICY_SPACE} --unwarn
EOF
