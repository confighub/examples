#!/usr/bin/env bash
# demo-setup.sh — Seed the cost-estimator demo fleet in ConfigHub, estimate the
# monthly cost of every workload, and let the guardrails gate the over-budget.
#
# Layout (mirrors the sec-scanner example):
#
#   cost-demo-policy    Triggers (the cost guardrail pack) + Filters. No Units.
#   cost-demo-base      Workload Units (Deployments/StatefulSet) with resource requests.
#   cost-demo-dev       "Cluster" Space (Environment=Dev)     — clones + planted violations
#   cost-demo-staging   "Cluster" Space (Environment=Staging) — clones
#   cost-demo-prod      "Cluster" Space (Environment=Prod)    — clones, approval required
#
# Cluster Spaces select the guardrail Triggers via a Filter in the policy Space
# (TriggerFilterID pattern), so policy is defined once and enforced everywhere.
#
# Guardrails:
#   valid-schemas      vet-schemas   — Kubernetes schema validation
#   requests-required  vet-celexpr   — every container must declare cpu+memory requests
#   within-budget      vet-celexpr   — block workloads the estimator flagged OVER budget
#   require-approval   vet-approvedby 1 (prod only)
#
# The within-budget gate is data-driven: the custom estimator (estimator/) reads
# each Unit's resource requests, costs them against a static price book, and
# writes the monthly estimate + a budget verdict back onto the Unit as
# annotations. The Trigger then gates whatever it marked OVER. Config (the
# requests) and the verdict both live as data.
#
# These are "paper clusters": ConfigHub Spaces only, no Targets/Workers — nothing
# deploys to a live cluster, and nothing touches the outside world.
#
# Prerequisites:
#   - cub CLI installed + authenticated (cub auth login)
#   - go (to build the estimator)
#
# Usage:
#   ./demo-setup.sh                  # create everything (idempotent; safe to re-run)
#   ./demo-setup.sh --explain        # print the plan, mutate nothing
#   ./demo-setup.sh --explain-json   # print the plan as JSON, mutate nothing
#   ./demo-setup.sh --no-estimate    # seed ConfigHub only; skip the cost estimate
#
# Environment variables:
#   PREFIX   Space slug prefix (default: cost-demo)
#   CUB      Path to cub binary (default: cub on PATH)

set -euo pipefail

PREFIX="${PREFIX:-cost-demo}"
EXAMPLE_NAME="cost-estimator"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFESTS="${SCRIPT_DIR}/manifests"
PRICEBOOK="${SCRIPT_DIR}/pricing/pricebook.json"

cub="${CUB:-cub}"

POLICY_SPACE="${PREFIX}-policy"
BASE_SPACE="${PREFIX}-base"
CLUSTER_SPACES=("${PREFIX}-dev" "${PREFIX}-staging" "${PREFIX}-prod")
DEV_SPACE="${PREFIX}-dev"
WORKLOADS=(frontend api cache db)
VIOLATIONS=(oversized-analytics no-requests-web)

# Guardrail CEL expressions. Validated offline against the manifests with:
#   cub function local <manifest> vet-celexpr '<expr>' --toolchain Kubernetes/YAML
REQUESTS_REQUIRED="!(r.kind in ['Deployment','StatefulSet']) || r.spec.template.spec.containers.all(c, has(c.resources) && has(c.resources.requests) && 'cpu' in c.resources.requests && 'memory' in c.resources.requests)"
WITHIN_BUDGET="!(r.kind in ['Deployment','StatefulSet']) || !has(r.metadata.annotations) || !('cost-estimator.confighub.com/budget-status' in r.metadata.annotations) || r.metadata.annotations['cost-estimator.confighub.com/budget-status'] != 'OVER'"

# ── Explain modes (no mutation) ───────────────────────────────────────────────

explain() {
  cat <<EOF
cost-estimator setup plan
=========================

Model: workload cost managed as data. Resource requests live in ConfigHub Units;
a custom estimator costs each workload against a static, versioned price book and
writes the monthly estimate + a budget verdict back as data; guardrails gate the
over-budget.

    ${POLICY_SPACE}            ${BASE_SPACE}
    (guardrail Triggers          (workload Units with requests:
     + Filters, no Units)         frontend, api, cache, db)
            |                           |  clone (upstream/downstream)
            | TriggerFilterID           v
            +----------->  ${PREFIX}-dev     (Environment=Dev;     + planted violations)
            +----------->  ${PREFIX}-staging (Environment=Staging)
            +----------->  ${PREFIX}-prod    (Environment=Prod;    + approval required)

       pricebook.json (CPU/mem/storage rates + per-env budgets)
            ▲
            │ cost = requests × replicas × rates  (+ storage)
       estimator ── read requests ─▶ cost ─▶ budget verdict ─▶ write back to Units

Will create (idempotently):
  - 5 Spaces: ${POLICY_SPACE}, ${BASE_SPACE}, ${CLUSTER_SPACES[*]}
  - 4 Triggers in ${POLICY_SPACE} (Pack=cost-guardrails):
      valid-schemas       vet-schemas
      requests-required   vet-celexpr (block workloads with no cpu/memory requests)
      within-budget       vet-celexpr (block workloads estimated OVER budget)
      require-approval    vet-approvedby 1 (prod only)
  - 2 Trigger Filters: cost-guardrails (Scope=all), cost-guardrails-prod (incl. approval)
  - 4 workload Units in ${BASE_SPACE}, cloned into each of the 3 cluster Spaces (12 clones)
  - 2 planted violations in ${PREFIX}-dev:
      oversized-analytics  10× 4cpu/16Gi → ~\$1,372/mo > \$500 dev budget → gated (within-budget)
      no-requests-web      no resource requests          → gated (requests-required, static)

Then: build + run the estimator over the fleet with --write-back so the
within-budget gate fires on the over-budget Units, and publish a pricebook-status
Unit into ${POLICY_SPACE}.

Mutates: ConfigHub (Spaces/Units) only. No Kubernetes Targets, Workers, or live
deploys; no external network.
EOF
}

explain_json() {
  local spaces_json units_json
  spaces_json=$(printf '"%s",' "$POLICY_SPACE" "$BASE_SPACE" "${CLUSTER_SPACES[@]}")
  units_json=$(printf '"%s",' "${WORKLOADS[@]}" "${VIOLATIONS[@]}")
  cat <<EOF
{
  "example_name": "${EXAMPLE_NAME}",
  "mutates": true,
  "mutates_confighub": true,
  "mutates_live_infra": false,
  "spaces": [${spaces_json%,}],
  "units": [${units_json%,}],
  "pricing_model": "static price book (pricing/pricebook.json), CPU + memory + storage",
  "notes": {
    "workloads_cloned_into": ["${PREFIX}-dev", "${PREFIX}-staging", "${PREFIX}-prod"],
    "violations_space": "${PREFIX}-dev",
    "expected_apply_gates": {
      "oversized-analytics": "within-budget",
      "no-requests-web": "requests-required"
    }
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

DO_ESTIMATE=1
case "${1:-}" in
  --explain) explain; exit 0 ;;
  --explain-json) explain_json; exit 0 ;;
  --no-estimate) DO_ESTIMATE=0 ;;
  "") ;;
  *) echo "Unknown argument: $1 (supported: --explain, --explain-json, --no-estimate)" >&2; exit 2 ;;
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
for f in "${WORKLOADS[@]/#/${MANIFESTS}/workloads/}" "${VIOLATIONS[@]/#/${MANIFESTS}/violations/}"; do
  [[ -f "${f}.yaml" ]] || { echo "ERROR: manifest not found: ${f}.yaml" >&2; exit 1; }
done

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
ensure_space "$POLICY_SPACE" --label app=cost-estimator --label role=policy

create_trigger() { # slug scope description function [args...]
  local slug="$1" scope="$2" desc="$3"; shift 3
  if trigger_exists "$POLICY_SPACE" "$slug"; then
    note "  trigger ${slug} exists, skipping"; ((skipped+=1))
  else
    $cub trigger create --space "$POLICY_SPACE" \
      --label Pack=cost-guardrails --label "Scope=${scope}" \
      --description "$desc" \
      "$slug" Mutation Kubernetes/YAML "$@" >/dev/null
    note "  created trigger ${slug}"; ((created+=1))
  fi
}

create_trigger valid-schemas all \
  "Validates Kubernetes resource schemas with kubeconform. Fix: correct the field names/types reported." \
  vet-schemas

create_trigger requests-required all \
  "Blocks workloads whose containers omit cpu/memory requests (uncostable + a scheduling hazard). Fix: add resources.requests.cpu and .memory." \
  vet-celexpr "$REQUESTS_REQUIRED"

create_trigger within-budget all \
  "Blocks workloads the estimator flagged OVER their environment budget (cost-estimator.confighub.com/budget-status). Fix: cut replicas/requests, or raise the budget, then re-estimate." \
  vet-celexpr "$WITHIN_BUDGET"

create_trigger require-approval prod \
  "Requires one approval before prod workload changes can be applied. Fix: have a reviewer approve the Unit." \
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

ensure_filter cost-guardrails      "Labels.Pack = 'cost-guardrails' AND Labels.Scope = 'all'"
ensure_filter cost-guardrails-prod "Labels.Pack = 'cost-guardrails'"

# ── 2. Base Space: workload Units with resource requests ──────────────────────

note "Base Space: ${BASE_SPACE}"
ensure_space "$BASE_SPACE" --label app=cost-estimator --label role=base \
  --trigger-filter "${POLICY_SPACE}/cost-guardrails"

for w in "${WORKLOADS[@]}"; do
  if unit_exists "$BASE_SPACE" "$w"; then
    note "  unit ${w} exists, skipping"; ((skipped+=1))
  else
    # Feed config via stdin ("-") so no local source path is recorded.
    $cub unit create --space "$BASE_SPACE" "$w" - \
      --label app=cost-estimator --label "workload=${w}" \
      --change-desc "Seed ${w} workload with resource requests" \
      < "${MANIFESTS}/workloads/${w}.yaml" >/dev/null
    note "  created unit ${w}"; ((created+=1))
  fi
done

# ── 3. Cluster Spaces: clones of the base workloads ───────────────────────────
# Region drives the price multiplier; Environment selects the budget (both are
# well-known ConfigHub fleet labels the estimator reads).

cluster_env()    { case "$1" in *-dev) echo Dev ;; *-staging) echo Staging ;; *-prod) echo Prod ;; esac; }
cluster_region() { case "$1" in *-dev) echo us-west-2 ;; *-staging) echo us-east-2 ;; *-prod) echo us-east-1 ;; esac; }
cluster_filter() { case "$1" in *-prod) echo cost-guardrails-prod ;; *) echo cost-guardrails ;; esac; }

for space in "${CLUSTER_SPACES[@]}"; do
  env="$(cluster_env "$space")"
  region="$(cluster_region "$space")"
  note "Cluster Space: ${space} (Environment=${env}, Region=${region})"
  ensure_space "$space" \
    --label app=cost-estimator --label "Environment=${env}" --label "Region=${region}" \
    --trigger-filter "${POLICY_SPACE}/$(cluster_filter "$space")"

  for w in "${WORKLOADS[@]}"; do
    if unit_exists "$space" "$w"; then
      note "  unit ${w} exists, skipping"; ((skipped+=1))
    else
      $cub unit create --space "$space" "$w" \
        --upstream-unit "$w" --upstream-space "$BASE_SPACE" \
        --label app=cost-estimator --label "workload=${w}" \
        --label "Environment=${env}" --label "Region=${region}" \
        --change-desc "Clone ${w} workload from ${BASE_SPACE}" >/dev/null
      note "  cloned unit ${w}"; ((created+=1))
    fi
  done
done

# ── 4. Planted violations in dev ──────────────────────────────────────────────

note "Planted violations in ${DEV_SPACE}"
dev_env="$(cluster_env "$DEV_SPACE")"
dev_region="$(cluster_region "$DEV_SPACE")"
for v in "${VIOLATIONS[@]}"; do
  if unit_exists "$DEV_SPACE" "$v"; then
    note "  unit ${v} exists, skipping"; ((skipped+=1))
  else
    # Feed config via stdin ("-") so no local source path is recorded.
    $cub unit create --space "$DEV_SPACE" "$v" - \
      --label app=cost-estimator --label imported=true \
      --label "Environment=${dev_env}" --label "Region=${dev_region}" \
      --change-desc "Planted demo violation: ${v}" \
      < "${MANIFESTS}/violations/${v}.yaml" >/dev/null
    note "  created unit ${v}"; ((created+=1))
  fi
done

# ── 5. Estimate the fleet ─────────────────────────────────────────────────────

if (( DO_ESTIMATE )); then
  note ""
  note "Estimating the fleet (price book + custom estimator)"
  estimator_bin="${SCRIPT_DIR}/estimator/costest"
  if [[ ! -x "$estimator_bin" ]]; then
    note "  building estimator..."
    ( cd "${SCRIPT_DIR}/estimator" && go build -o costest . )
  fi
  # The estimator talks to the ConfigHub REST API directly (like the web app does),
  # so bridge this cub session's server URL + token to it via the environment.
  export CONFIGHUB_URL="${CONFIGHUB_URL:-$($cub context get 2>/dev/null | awk '/Server URL/{print $NF}')}"
  export CONFIGHUB_TOKEN="${CONFIGHUB_TOKEN:-$($cub auth get-token 2>/dev/null)}"
  note "  costing workloads across ${PREFIX}-* and writing estimates back..."
  "$estimator_bin" estimate-fleet --space "${PREFIX}-*" --write-back \
    --status-space "$POLICY_SPACE" --pricebook "$PRICEBOOK"
else
  note ""
  note "Skipped estimate (--no-estimate). The within-budget gate will not fire until you run:"
  note "  (cd estimator && go build -o costest .)"
  note "  ./estimator/costest estimate-fleet --space '${PREFIX}-*' --write-back --pricebook pricing/pricebook.json"
fi

# ── Summary ───────────────────────────────────────────────────────────────────

cat <<EOF

Done. Created ${created} entities, skipped ${skipped} existing.

Inspect the result:
  $cub unit list --space "${PREFIX}-*" --where "Labels.app = 'cost-estimator'"
  $cub unit get oversized-analytics --space ${DEV_SPACE} -o jq=".Unit.ApplyGates"
  ./estimator/costest inventory --space "${PREFIX}-*" --pricebook pricing/pricebook.json

Next: ./demo-verify.sh confirms the layout, the gate matrix, and the estimates.
EOF
