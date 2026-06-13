#!/usr/bin/env bash
# demo-setup.sh — Seed the rbac-manager demo fleet in ConfigHub
#
# Creates a multi-cluster Kubernetes RBAC layout managed as configuration data:
#
#   rbac-demo-policy    Triggers (the RBAC guardrail pack) + Filters. No Units.
#   rbac-demo-base      Canonical persona Units: developer, operator, viewer, ci.
#   rbac-demo-dev       "Cluster" Space (env=dev,     region=use1) — persona clones
#   rbac-demo-staging   "Cluster" Space (env=staging, region=use1) — persona clones
#   rbac-demo-prod      "Cluster" Space (env=prod,    region=use2) — persona clones
#
# Cluster Spaces select the guardrail Triggers via a Filter in the policy
# Space (TriggerFilterID pattern), so policy is defined once and enforced
# everywhere. Prod additionally requires approval (vet-approvedby).
#
# The dev Space gets three planted violations so audit findings and Apply
# Gates demo immediately:
#   - legacy-wildcard-admin   ClusterRole with wildcard rules  → gated (no-wildcards)
#   - oncall-breakglass       ClusterRoleBinding cluster-admin → gated (no-cluster-admin-binding)
#   - orphaned-grafana-binding RoleBinding to a missing Role   → app-side audit finding (no gate)
#
# The dev "developer" persona also diverges from base (gains the "delete"
# verb) via a server-side yq-i mutation, to demo variant divergence.
#
# These are "paper clusters": Spaces only, no Targets/Workers, so nothing is
# deployed to live infrastructure. (A --kind mode binding real kind-cluster
# Targets is planned but not yet implemented.)
#
# Prerequisites:
#   - cub CLI installed: https://docs.confighub.com/get-started/setup/#install-the-cli
#   - Authenticated: cub auth login
#
# Usage:
#   ./demo-setup.sh                  # create everything (idempotent; safe to re-run)
#   ./demo-setup.sh --explain        # print the plan, mutate nothing
#   ./demo-setup.sh --explain-json   # print the plan as JSON, mutate nothing
#
# Environment variables:
#   PREFIX   Space slug prefix (default: rbac-demo)
#   CUB      Path to cub binary (default: cub on PATH)

set -euo pipefail

PREFIX="${PREFIX:-rbac-demo}"
EXAMPLE_NAME="rbac-manager"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS="${SCRIPT_DIR}/manifests"

cub="${CUB:-cub}"

POLICY_SPACE="${PREFIX}-policy"
BASE_SPACE="${PREFIX}-base"
CLUSTER_SPACES=("${PREFIX}-dev" "${PREFIX}-staging" "${PREFIX}-prod")
PERSONAS=(developer operator viewer ci)
VIOLATIONS=(legacy-wildcard-admin orphaned-grafana-binding breakglass-cluster-admin)

# Guardrail CEL expressions. Validated offline against the manifests in this
# example with: cub function local <manifest> vet-celexpr '<expr>'
# Personas pass all three; legacy-wildcard-admin trips NO_WILDCARDS;
# breakglass-cluster-admin trips NO_CLUSTER_ADMIN.
NO_WILDCARDS="!(r.kind in ['Role', 'ClusterRole']) || !has(r.rules) || !r.rules.exists(rule, (has(rule.verbs) && rule.verbs.exists(v, v == '*')) || (has(rule.resources) && rule.resources.exists(x, x == '*')) || (has(rule.apiGroups) && rule.apiGroups.exists(g, g == '*')))"
NO_ESCALATION="!(r.kind in ['Role', 'ClusterRole']) || !has(r.rules) || !r.rules.exists(rule, has(rule.verbs) && rule.verbs.exists(v, v in ['escalate', 'bind', 'impersonate']))"
NO_CLUSTER_ADMIN="r.kind != 'ClusterRoleBinding' || r.roleRef.name != 'cluster-admin'"

# ── Explain modes (no mutation) ───────────────────────────────────────────────

explain() {
  cat <<EOF
rbac-manager setup plan
=======================

Model: Kubernetes RBAC managed as data, one Space per cluster, policy
enforced centrally via Triggers + Apply Gates.

    ${POLICY_SPACE}            ${BASE_SPACE}
    (guardrail Triggers        (canonical persona Units:
     + Filters, no Units)       developer, operator, viewer, ci)
            |                          |  clone (upstream/downstream)
            | TriggerFilterID          v
            +----------->  ${PREFIX}-dev   (env=dev,     region=use1)
            +----------->  ${PREFIX}-staging (env=staging, region=use1)
            +----------->  ${PREFIX}-prod  (env=prod,    region=use2; + approval required)

Will create (idempotently):
  - 5 Spaces: ${POLICY_SPACE}, ${BASE_SPACE}, ${CLUSTER_SPACES[*]}
  - 5 Triggers in ${POLICY_SPACE} (Pack=rbac-guardrails):
      valid-rbac-schemas        vet-schemas
      no-wildcards              vet-celexpr (no * verbs/resources/apiGroups)
      no-privilege-escalation   vet-celexpr (no escalate/bind/impersonate)
      no-cluster-admin-binding  vet-celexpr (no cluster-admin ClusterRoleBindings)
      require-approval          vet-approvedby 1 (prod only)
  - 2 Trigger Filters: rbac-guardrails (Scope=all), rbac-guardrails-prod (all incl. approval)
  - 4 persona Units in ${BASE_SPACE}, cloned into each of the 3 cluster Spaces (12 clones)
  - 1 divergence: ${PREFIX}-dev/developer gains the "delete" verb (server-side yq-i)
  - 3 planted violations in ${PREFIX}-dev (2 gated, 1 app-side audit finding)

Mutates: ConfigHub only. No Targets, no Workers, no live infrastructure.
EOF
}

explain_json() {
  local spaces_json units_json
  spaces_json=$(printf '"%s",' "$POLICY_SPACE" "$BASE_SPACE" "${CLUSTER_SPACES[@]}")
  units_json=$(printf '"%s",' "${PERSONAS[@]}" "${VIOLATIONS[@]}")
  cat <<EOF
{
  "example_name": "${EXAMPLE_NAME}",
  "mutates": true,
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "spaces": [${spaces_json%,}],
  "units": [${units_json%,}],
  "notes": {
    "personas_cloned_into": ["${PREFIX}-dev", "${PREFIX}-staging", "${PREFIX}-prod"],
    "violations_space": "${PREFIX}-dev",
    "expected_apply_gates": ["legacy-wildcard-admin", "breakglass-cluster-admin"],
    "expected_audit_finding_only": ["orphaned-grafana-binding"]
  },
  "evaluation_modes": {
    "fast_preview": {
      "mutates": false,
      "commands": ["./demo-setup.sh --explain", "./demo-setup.sh --explain-json | jq"]
    },
    "fast_operational_evaluation": {
      "mutates_confighub": true,
      "mutates_live_infra": false,
      "commands": ["./demo-setup.sh", "./demo-verify.sh"],
      "stop_before_cleanup": true
    }
  }
}
EOF
}

case "${1:-}" in
  --explain) explain; exit 0 ;;
  --explain-json) explain_json; exit 0 ;;
  "") ;;
  *) echo "Unknown argument: $1 (supported: --explain, --explain-json)" >&2; exit 2 ;;
esac

# ── Preflight ─────────────────────────────────────────────────────────────────

if ! command -v "$cub" &>/dev/null; then
  echo "ERROR: cub not found. Install it from https://docs.confighub.com/get-started/setup/#install-the-cli" >&2
  exit 1
fi

# Verify auth by hitting the server, not just local session state.
if ! $cub space list --quiet &>/dev/null; then
  echo "ERROR: Cannot reach ConfigHub (not authenticated?). Run: $cub auth login" >&2
  exit 1
fi

for f in "${PERSONAS[@]/#/${MANIFESTS}/personas/}" "${VIOLATIONS[@]/#/${MANIFESTS}/violations/}"; do
  if [[ ! -f "${f}.yaml" ]]; then
    echo "ERROR: manifest not found: ${f}.yaml" >&2
    exit 1
  fi
done

# ── Helpers ───────────────────────────────────────────────────────────────────

created=0
skipped=0

note() { printf '%s\n' "$*"; }

space_exists()   { $cub space get "$1" --quiet &>/dev/null; }
unit_exists()    { $cub unit get "$2" --space "$1" --quiet &>/dev/null; }
trigger_exists() { $cub trigger get "$2" --space "$1" --quiet &>/dev/null; }
filter_exists()  { $cub filter get "$2" --space "$1" --quiet &>/dev/null; }

ensure_space() { # slug, extra flags...
  local slug="$1"; shift
  if space_exists "$slug"; then
    note "  space ${slug} exists, skipping"; ((skipped+=1))
  else
    $cub space create "$slug" "$@" >/dev/null
    note "  created space ${slug}"; ((created+=1))
  fi
}

# ── 1. Policy Space: guardrail Triggers + Filters ─────────────────────────────

note "Policy Space: ${POLICY_SPACE}"
ensure_space "$POLICY_SPACE" --label app=rbac-manager --label role=policy

create_trigger() { # slug scope description function [args...]
  local slug="$1" scope="$2" desc="$3"; shift 3
  if trigger_exists "$POLICY_SPACE" "$slug"; then
    note "  trigger ${slug} exists, skipping"; ((skipped+=1))
  else
    $cub trigger create --space "$POLICY_SPACE" \
      --label Pack=rbac-guardrails --label "Scope=${scope}" \
      --description "$desc" \
      "$slug" Mutation Kubernetes/YAML "$@" >/dev/null
    note "  created trigger ${slug}"; ((created+=1))
  fi
}

create_trigger valid-rbac-schemas all \
  "Validates Kubernetes resource schemas with kubeconform. Fix: correct the field names/types reported in the failure details." \
  vet-schemas

create_trigger no-wildcards all \
  "Blocks Roles/ClusterRoles with wildcard verbs, resources, or apiGroups. Fix: enumerate the specific verbs/resources the role needs." \
  vet-celexpr "$NO_WILDCARDS"

create_trigger no-privilege-escalation all \
  "Blocks Roles/ClusterRoles granting escalate, bind, or impersonate. Fix: remove these verbs; they allow privilege escalation." \
  vet-celexpr "$NO_ESCALATION"

create_trigger no-cluster-admin-binding all \
  "Blocks ClusterRoleBindings to cluster-admin. Fix: bind a scoped role instead, or use the approval-gated break-glass flow." \
  vet-celexpr "$NO_CLUSTER_ADMIN"

create_trigger require-approval prod \
  "Requires one approval before prod RBAC changes can be applied. Fix: have a reviewer approve the Unit." \
  vet-approvedby 1

ensure_filter() { # slug where
  local slug="$1" where="$2"
  if filter_exists "$POLICY_SPACE" "$slug"; then
    note "  filter ${slug} exists, skipping"; ((skipped+=1))
  else
    $cub filter create --space "$POLICY_SPACE" "$slug" Trigger --where-field "$where" >/dev/null
    note "  created filter ${slug}"; ((created+=1))
  fi
}

ensure_filter rbac-guardrails      "Labels.Pack = 'rbac-guardrails' AND Labels.Scope = 'all'"
ensure_filter rbac-guardrails-prod "Labels.Pack = 'rbac-guardrails'"

# ── 2. Base Space: canonical persona Units ────────────────────────────────────

note "Base Space: ${BASE_SPACE}"
ensure_space "$BASE_SPACE" --label app=rbac-manager --label role=base \
  --trigger-filter "${POLICY_SPACE}/rbac-guardrails"

for persona in "${PERSONAS[@]}"; do
  if unit_exists "$BASE_SPACE" "$persona"; then
    note "  unit ${persona} exists, skipping"; ((skipped+=1))
  else
    $cub unit create --space "$BASE_SPACE" "$persona" \
      "${MANIFESTS}/personas/${persona}.yaml" \
      --label app=rbac-manager --label "persona=${persona}" \
      --change-desc "Seed canonical ${persona} persona" >/dev/null
    note "  created unit ${persona}"; ((created+=1))
  fi
done

# ── 3. Cluster Spaces: clones of the base personas ────────────────────────────

cluster_env()    { case "$1" in *-dev) echo dev ;; *-staging) echo staging ;; *-prod) echo prod ;; esac; }
cluster_region() { case "$1" in *-prod) echo use2 ;; *) echo use1 ;; esac; }
cluster_filter() { case "$1" in *-prod) echo rbac-guardrails-prod ;; *) echo rbac-guardrails ;; esac; }

for space in "${CLUSTER_SPACES[@]}"; do
  env="$(cluster_env "$space")"
  note "Cluster Space: ${space} (env=${env}, region=$(cluster_region "$space"))"
  ensure_space "$space" \
    --label app=rbac-manager --label "env=${env}" --label "region=$(cluster_region "$space")" \
    --trigger-filter "${POLICY_SPACE}/$(cluster_filter "$space")"

  for persona in "${PERSONAS[@]}"; do
    if unit_exists "$space" "$persona"; then
      note "  unit ${persona} exists, skipping"; ((skipped+=1))
    else
      $cub unit create --space "$space" "$persona" \
        --upstream-unit "$persona" --upstream-space "$BASE_SPACE" \
        --label app=rbac-manager --label "persona=${persona}" --label "env=${env}" \
        --change-desc "Clone ${persona} persona from ${BASE_SPACE}" >/dev/null
      note "  cloned unit ${persona}"; ((created+=1))
    fi
  done
done

# ── 4. Intentional divergence: dev developers may delete workloads ────────────

DEV_SPACE="${PREFIX}-dev"
# Strip full-line comments before grepping: the persona YAML's header comment
# mentions "delete", and unit data preserves comments.
if $cub unit data --space "$DEV_SPACE" developer 2>/dev/null \
    | grep -v '^[[:space:]]*#' | grep -qw 'delete'; then
  note "Divergence already present in ${DEV_SPACE}/developer, skipping"
  ((skipped+=1))
else
  note "Diverging ${DEV_SPACE}/developer from base (add delete verb, server-side yq-i)"
  $cub function do --space "$DEV_SPACE" --unit developer \
    --change-desc "Dev-only divergence: developers may delete workloads in dev" \
    -- yq-i 'select(.kind == "ClusterRole").rules[0].verbs += ["delete"]' >/dev/null
  ((created+=1))
fi

# ── 5. Planted violations in dev ──────────────────────────────────────────────

note "Planted violations in ${DEV_SPACE}"
for violation in "${VIOLATIONS[@]}"; do
  if unit_exists "$DEV_SPACE" "$violation"; then
    note "  unit ${violation} exists, skipping"; ((skipped+=1))
  else
    $cub unit create --space "$DEV_SPACE" "$violation" \
      "${MANIFESTS}/violations/${violation}.yaml" \
      --label app=rbac-manager --label imported=true \
      --change-desc "Planted demo violation: ${violation}" >/dev/null
    note "  created unit ${violation}"; ((created+=1))
  fi
done

# ── Summary ───────────────────────────────────────────────────────────────────

cat <<EOF

Done. Created ${created} entities, skipped ${skipped} existing.

Inspect the result:
  $cub unit list --space ${DEV_SPACE}
  $cub unit get legacy-wildcard-admin --space ${DEV_SPACE} -o jq=".Unit.ApplyGates"
  $cub unit list --space "*" --where "Labels.persona = 'developer'"

Next: ./demo-verify.sh confirms the seeded fleet, including that the planted
violations carry the expected Apply Gates.
EOF
