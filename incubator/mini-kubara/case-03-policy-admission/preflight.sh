#!/usr/bin/env bash
#
# Mini-Kubara Case 03 preflight.
#
# Read-only. Scans the local fixture policy and deployment variants, proves
# the bad workload violates the policy by construction, proves the fixed
# workload satisfies the policy by construction, and states the WATCH -> BLOCK
# tripwire BEFORE any live mutation. This helper does not create a cluster,
# install Kyverno, apply the policy, or sync Argo Applications. Gate A still
# requires explicit operator approval.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="${FIXTURES_DIR:-${SCRIPT_DIR}/fixtures}"
POLICIES_DIR="${POLICIES_DIR:-${FIXTURES_DIR}/policies}"
WORKLOADS_DIR="${WORKLOADS_DIR:-${FIXTURES_DIR}/workloads}"
APPSET_FILE="${APPSET_FILE:-${FIXTURES_DIR}/applicationsets/mini-policy.yaml}"
POLICY_FILE="${POLICY_FILE:-${POLICIES_DIR}/require-safe-container.yaml}"
BAD_FILE="${BAD_FILE:-${WORKLOADS_DIR}/deployment-bad.yaml}"
FIXED_FILE="${FIXED_FILE:-${WORKLOADS_DIR}/deployment-fixed.yaml}"

usage() {
  cat <<EOF
Usage: $0 [--explain]

Read-only preflight for Mini-Kubara Case 03: Policy Admission.

What this helper does:
  - confirms the fixture layout (policy, bad deployment, fixed deployment,
    ApplicationSet) is present;
  - proves the bad Deployment lacks securityContext.runAsNonRoot and
    securityContext.allowPrivilegeEscalation on every container, so Pod
    admission by the require-safe-container policy would be rejected;
  - proves the fixed Deployment sets both fields on every container, so
    Pod admission would succeed;
  - checks the Kyverno policy is scoped (Enforce, Fail, namespaced match on
    mini-policy) and names the admission webhook surface to inspect at
    Gate A;
  - checks the ApplicationSet defaults to the bad workload so the
    BLOCK is deterministic on first sync;
  - states the WATCH -> BLOCK tripwire and the recovery path before any
    mutation.

What this helper does NOT do:
  - it does not create a kind cluster;
  - it does not install Kyverno, ConfigHub workers, or targets;
  - it does not apply the policy, workloads, or ApplicationSet;
  - it does not mutate ConfigHub state;
  - it does not relax or delete any policy.

Environment overrides:
  FIXTURES_DIR    (default <script>/fixtures)
  POLICIES_DIR    (default <fixtures>/policies)
  WORKLOADS_DIR   (default <fixtures>/workloads)
  APPSET_FILE     (default <fixtures>/applicationsets/mini-policy.yaml)
  POLICY_FILE     (default <policies>/require-safe-container.yaml)
  BAD_FILE        (default <workloads>/deployment-bad.yaml)
  FIXED_FILE      (default <workloads>/deployment-fixed.yaml)
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

if [[ "${1:-}" == "--explain" ]]; then
  usage
  cat <<'EOF'

Why this preflight exists
-------------------------
Fail-closed admission policies (Kyverno with Enforce + Fail, or similar
validating webhooks) reject non-compliant resources at admission time. The
Pod creation fails, the ReplicaSet records the webhook error, and the
Deployment is stuck with zero Ready replicas. The correct response is not a
retry. It is a classification (WATCH -> BLOCK) and a recovery decision:

  - fix the governed desired state so the workload satisfies the policy; or
  - explicitly approve a dev-only policy relaxation for the exact namespace.

Retrying the same rejected manifest, disabling the policy globally, or
patching the live cluster while the governed desired state still violates
the policy are all stop conditions for this case.

What the helper proves by construction
--------------------------------------
The helper reads the fixture YAML files directly and extracts container
securityContext fields. It does not need a cluster. If the bad Deployment
contains any container without runAsNonRoot=true or without
allowPrivilegeEscalation=false, the fixture is already guaranteed to fail
admission under the policy fixture. If the fixed Deployment sets both fields
on every container, the fixture is guaranteed to pass.

This is the structural proof a fresh Claude session needs before Gate A:
"the BLOCK is real, I can point to the exact missing fields, and the recovery
variant sets them."

Mutation posture
----------------
This helper is intentionally read-only. It does not install Kyverno, run
kubectl apply, sync the ApplicationSet, or make ConfigHub writes. The live
admission test and the recovery gate are reserved for Gate A and Gate B,
each approved explicitly by the operator.
EOF
  exit 0
fi

if [[ $# -gt 0 ]]; then
  echo "ERROR: unknown argument: $1" >&2
  usage >&2
  exit 2
fi

for f in "${POLICY_FILE}" "${BAD_FILE}" "${FIXED_FILE}" "${APPSET_FILE}"; do
  if [[ ! -f "$f" ]]; then
    echo "ERROR: required fixture not found: $f" >&2
    echo "Hint: the Case 03 prep packet should be present before running this helper." >&2
    exit 2
  fi
done

# Extract every container's securityContext subtree from a Deployment fixture
# and report whether runAsNonRoot=true and allowPrivilegeEscalation=false are
# both set on every container. Done with awk because we cannot assume a YAML
# parser is installed and the fixtures here are deliberately simple.
analyse_deployment() {
  local file="$1"
  awk '
    BEGIN {
      in_containers = 0
      container_count = 0
      current = ""
      run_as_non_root = 0
      no_priv_escalation = 0
    }

    # Track entry into the containers: list under spec.template.spec.containers.
    # The fixtures are small and hand-written, so structural indentation is
    # stable at 6 spaces for the containers array.
    /^      containers:/ {
      in_containers = 1
      next
    }

    # Leave containers: block when a sibling key at the same indent appears.
    in_containers && /^      [a-zA-Z]/ && !/^      containers:/ && !/^      - / && !/^        / {
      in_containers = 0
    }

    in_containers && /^        - name:/ {
      if (current != "") {
        print current, run_as_non_root, no_priv_escalation
      }
      container_count += 1
      current = $3
      run_as_non_root = 0
      no_priv_escalation = 0
      next
    }

    in_containers && /^ {12}runAsNonRoot:[[:space:]]*true/ {
      run_as_non_root = 1
    }
    in_containers && /^ {12}allowPrivilegeEscalation:[[:space:]]*false/ {
      no_priv_escalation = 1
    }

    END {
      if (current != "") {
        print current, run_as_non_root, no_priv_escalation
      }
      print "TOTAL", container_count
    }
  ' "$file"
}

summarise_variant() {
  local file="$1"
  local variant="$2"
  local parsed
  parsed="$(analyse_deployment "$file")"
  local total
  total="$(echo "$parsed" | awk '$1=="TOTAL" {print $2}')"
  local containers
  containers="$(echo "$parsed" | awk '$1!="TOTAL" {print}')"
  local ok_count=0
  local bad_count=0
  local missing_details=""
  if [[ -n "$containers" ]]; then
    while read -r name run_as_non_root no_priv_escalation; do
      if [[ "$run_as_non_root" == "1" && "$no_priv_escalation" == "1" ]]; then
        ok_count=$((ok_count + 1))
      else
        bad_count=$((bad_count + 1))
        local missing=""
        [[ "$run_as_non_root" == "0" ]] && missing="${missing}runAsNonRoot"
        [[ "$no_priv_escalation" == "0" ]] && { [[ -n "$missing" ]] && missing="$missing+"; missing="${missing}allowPrivilegeEscalation"; }
        missing_details="${missing_details}${name}(${missing}) "
      fi
    done <<< "$containers"
  fi
  echo "${variant}|${total}|${ok_count}|${bad_count}|${missing_details% }"
}

bad_summary="$(summarise_variant "$BAD_FILE" BAD)"
fixed_summary="$(summarise_variant "$FIXED_FILE" FIXED)"

bad_total="$(echo "$bad_summary" | cut -d'|' -f2)"
bad_ok="$(echo "$bad_summary" | cut -d'|' -f3)"
bad_bad="$(echo "$bad_summary" | cut -d'|' -f4)"
bad_missing="$(echo "$bad_summary" | cut -d'|' -f5)"

fixed_total="$(echo "$fixed_summary" | cut -d'|' -f2)"
fixed_ok="$(echo "$fixed_summary" | cut -d'|' -f3)"
fixed_bad="$(echo "$fixed_summary" | cut -d'|' -f4)"
fixed_missing="$(echo "$fixed_summary" | cut -d'|' -f5)"

# Policy shape checks.
policy_enforce="no"
policy_failpolicy="no"
policy_namespace_scope="no"
policy_rules=0
if grep -q 'validationFailureAction:[[:space:]]*Enforce' "$POLICY_FILE"; then
  policy_enforce="yes"
fi
if grep -q 'failurePolicy:[[:space:]]*Fail' "$POLICY_FILE"; then
  policy_failpolicy="yes"
fi
if grep -q -- '- mini-policy' "$POLICY_FILE"; then
  policy_namespace_scope="yes"
fi
policy_rules="$(grep -c '^    - name:' "$POLICY_FILE" || true)"

# ApplicationSet shape checks.
appset_default_bad="no"
appset_automated="unknown"
if grep -q 'include:[[:space:]]*deployment-bad.yaml' "$APPSET_FILE"; then
  appset_default_bad="yes"
fi
if grep -q 'automated:' "$APPSET_FILE"; then
  appset_automated="yes"
else
  appset_automated="no"
fi

echo ""
echo "Mini-Kubara Case 03: Policy Admission preflight (read-only)"
echo "==========================================================="
echo ""
echo "Fixtures root:        ${FIXTURES_DIR}"
echo "Policy fixture:       ${POLICY_FILE#${FIXTURES_DIR}/}"
echo "Bad Deployment:       ${BAD_FILE#${FIXTURES_DIR}/}"
echo "Fixed Deployment:     ${FIXED_FILE#${FIXTURES_DIR}/}"
echo "ApplicationSet:       ${APPSET_FILE#${FIXTURES_DIR}/}"
echo ""
printf '%-10s %-40s %12s %s\n' LANE FIXTURE CONTAINERS RESULT
printf '%-10s %-40s %12s %s\n' ---- ------- ---------- ------

if (( bad_bad > 0 )); then
  printf '%-10s %-40s %12s %s\n' "POLICY-READ" "deployment-bad.yaml" "${bad_total}" \
    "BLOCK by construction (missing: ${bad_missing:-none})"
else
  printf '%-10s %-40s %12s %s\n' "POLICY-READ" "deployment-bad.yaml" "${bad_total}" \
    "UNEXPECTED: bad fixture already compliant"
fi

if (( fixed_bad == 0 && fixed_total > 0 )); then
  printf '%-10s %-40s %12s %s\n' "POLICY-READ" "deployment-fixed.yaml" "${fixed_total}" \
    "OK by construction"
else
  printf '%-10s %-40s %12s %s\n' "POLICY-READ" "deployment-fixed.yaml" "${fixed_total}" \
    "BLOCK: fixed fixture still missing (${fixed_missing:-unknown})"
fi

echo ""
echo "Policy shape"
echo "------------"
echo "validationFailureAction=Enforce:        ${policy_enforce}"
echo "failurePolicy=Fail:                     ${policy_failpolicy}"
echo "match scope restricted to mini-policy:  ${policy_namespace_scope}"
echo "validate rules counted:                 ${policy_rules}"
echo ""
echo "ApplicationSet shape"
echo "--------------------"
echo "default include is deployment-bad.yaml: ${appset_default_bad}"
echo "syncPolicy.automated set:               ${appset_automated}"
echo ""

# Classify overall readiness.
fixture_ok="yes"
if (( bad_bad == 0 )); then fixture_ok="no"; fi
if (( fixed_bad != 0 )); then fixture_ok="no"; fi
if [[ "$policy_enforce" != "yes" || "$policy_failpolicy" != "yes" || "$policy_namespace_scope" != "yes" ]]; then
  fixture_ok="no"
fi
if [[ "$appset_default_bad" != "yes" ]]; then
  fixture_ok="no"
fi

if [[ "$fixture_ok" == "yes" ]]; then
  cat <<EOF
CONFIGHUB SAYS: ROUTE
Lane: CH-WRITE for governed desired-state fix at Gate B;
      SETUP for installing Kyverno + policy at Gate A;
      LIVE-WRITE only for an explicitly approved dev-only policy relaxation
Scope: Mini-Kubara Case 03 policy + one app (mini-policy namespace) only
Wrong move: retrying the rejected manifest; disabling policy silently;
  patching the live workload while ConfigHub desired state still violates
  the policy

WATCH tripwire:
  Kyverno pods running but policy not yet bound to the admission webhook
  is WATCH. Fail-closed rejection of an in-scope resource promotes WATCH
  to BLOCK. Admission webhook Service unreachable while failurePolicy=Fail
  is BLOCK before any intended mutation.

Admission surface to inspect at Gate A (read-only, no apply yet):
  - ValidatingWebhookConfiguration named kyverno-resource-validating-webhook-cfg
    (or equivalent) and its Service endpoints;
  - Kyverno controller Deployment and Pods in the kyverno namespace;
  - ClusterPolicy/require-safe-container .status.ready after install;
  - recent kyverno events on Pods in the mini-policy namespace.

Recovery preference (before any mutation):
  1. Fix governed desired state. Change the ApplicationSet Unit's
     generator include from deployment-bad.yaml to deployment-fixed.yaml
     through ConfigHub and let Argo reconcile.
  2. Only if the user explicitly requests a dev-only variant, add a namespace
     exclusion or reduce validationFailureAction to Audit on a named dev
     namespace. Never delete or weaken the policy globally.

STOP. This is a read-only preflight. Do not create a cluster, install
Kyverno, apply the policy, apply workloads, sync the ApplicationSet, or
make any ConfigHub write without an explicit Gate A approval.

Next gate (needs explicit Y/N):
  Gate A: install the policy and the intentionally bad app path in scope,
          observe the admission rejection, and classify WATCH -> BLOCK.
EOF
else
  cat <<EOF
CONFIGHUB SAYS: WARNING
Case 03 fixture does not pass by construction.

Required shape:
  - deployment-bad.yaml has at least one container missing runAsNonRoot=true
    or allowPrivilegeEscalation=false (current bad_containers_missing=${bad_bad});
  - deployment-fixed.yaml sets BOTH fields on every container
    (current fixed_containers_missing=${fixed_bad});
  - policy has validationFailureAction=Enforce (current ${policy_enforce}),
    failurePolicy=Fail (current ${policy_failpolicy}), and match scope
    restricted to the mini-policy namespace (current ${policy_namespace_scope});
  - ApplicationSet defaults to deployment-bad.yaml so the BLOCK is
    deterministic on first sync (current ${appset_default_bad}).

STOP before proposing Gate A. Fix the fixture first.
EOF
fi

exit 0
