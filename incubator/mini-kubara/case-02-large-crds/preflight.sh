#!/usr/bin/env bash
#
# Mini-Kubara Case 02 preflight.
#
# Read-only. Scans the local fixture CRDs, estimates annotation-size risk,
# and states the intended apply mode BEFORE any live mutation. This helper
# does not create a cluster, install Argo CD, apply CRDs, or make ConfigHub
# writes. Gate A still requires explicit operator approval.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURES_DIR="${FIXTURES_DIR:-${SCRIPT_DIR}/fixtures}"
CRDS_DIR="${CRDS_DIR:-${FIXTURES_DIR}/workloads/crds}"
APPSET_FILE="${APPSET_FILE:-${FIXTURES_DIR}/applicationsets/mini-large-crds.yaml}"
WIDGET_FILE="${WIDGET_FILE:-${FIXTURES_DIR}/resources/widget-example.yaml}"
CONTROLLER_FILE="${CONTROLLER_FILE:-${FIXTURES_DIR}/workloads/controller.yaml}"
ANNOTATION_LIMIT="${ANNOTATION_LIMIT:-262144}"

usage() {
  cat <<EOF
Usage: $0 [--explain]

Read-only preflight for Mini-Kubara Case 02: Large CRDs.

What this helper does:
  - counts fixture CRDs;
  - measures each CRD's serialized body size;
  - flags any CRD whose body already exceeds the ${ANNOTATION_LIMIT}-byte
    annotation limit, which makes kubectl client-side apply unsafe;
  - checks the Case 02 Argo hardening knobs:
    template.spec.syncPolicy is present, ServerSideApply=true,
    ClientSideApplyMigration=false, and the per-resource Argo sync-options
    annotation on the oversized CRD;
  - states the intended apply mode before any mutation;
  - recommends the explicit delivery path for the future live run.

What this helper does NOT do:
  - it does not create a kind cluster;
  - it does not install Argo CD, ConfigHub workers, or targets;
  - it does not apply CRDs, controllers, or custom resources to any cluster;
  - it does not mutate ConfigHub state.

Environment overrides:
  ANNOTATION_LIMIT (default 262144)
  FIXTURES_DIR     (default <script>/fixtures)
  CRDS_DIR         (default <fixtures>/workloads/crds)
  APPSET_FILE      (default <fixtures>/applicationsets/mini-large-crds.yaml)
  WIDGET_FILE      (default <fixtures>/resources/widget-example.yaml)
  CONTROLLER_FILE  (default <fixtures>/workloads/controller.yaml)
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
Large CRDs (cert-manager, external-secrets, and similar platform apps) can
exceed the Kubernetes metadata.annotations limit of 262144 bytes when
kubectl client-side apply stamps the full manifest into the
kubectl.kubernetes.io/last-applied-configuration annotation. When that
happens the API server rejects the CRD with:

    metadata.annotations: Too long: may not be more than 262144 bytes

The correct response is not a retry. It is a delivery-mode decision.
The safe paths are:

  - server-side apply: kubectl apply --server-side=true --force-conflicts,
    which does not stamp last-applied-configuration at all;
  - an Argo/Flux/ConfigHub sync option that forwards ServerSideApply=true
    to the controller;
  - for Argo, a per-resource
    argocd.argoproj.io/sync-options: ServerSideApply=true annotation on the
    oversized CRD itself.

Case 02's 2026-04-23 live pass proved the preflight was useful: the large CRD
failed with metadata.annotations: Too long when only the Application-level
ServerSideApply=true path was present. Argo's current sync-options docs say
SSA can be configured both at Application level and per resource, and
client-side apply migration is enabled by default unless
ClientSideApplyMigration=false is set. The hardened fixture therefore uses all
three explicit signals before rerun:

  - Application syncOptions: ServerSideApply=true
  - Application syncOptions: ClientSideApplyMigration=false
  - CRD annotation: argocd.argoproj.io/sync-options: ServerSideApply=true
  - ApplicationSet structure: syncPolicy is under template.spec, not source

What the helper measures
------------------------
Serialized YAML byte length per CRD file. If that length already exceeds
262144 bytes, client-side apply is guaranteed to fail: the JSON form that
kubectl stamps into the annotation is not smaller in any meaningful way.
A fixture CRD under the threshold is reported as OK but still routed
through the server-side path so the case exercises the full delivery
contract.

Mutation posture
----------------
This helper is intentionally read-only. It does not install CRDs, run
kubectl apply, or make ConfigHub writes. The live apply-mode decision is
reserved for Gate A, which is approved explicitly by the operator.
EOF
  exit 0
fi

if [[ $# -gt 0 ]]; then
  echo "ERROR: unknown argument: $1" >&2
  usage >&2
  exit 2
fi

if [[ ! -d "${FIXTURES_DIR}" ]]; then
  echo "ERROR: fixtures directory not found: ${FIXTURES_DIR}" >&2
  exit 2
fi
if [[ ! -d "${CRDS_DIR}" ]]; then
  echo "ERROR: CRD fixtures directory not found: ${CRDS_DIR}" >&2
  echo "Hint: run ${FIXTURES_DIR}/generate-stores-large.sh to regenerate the large CRD." >&2
  exit 2
fi

crd_files=()
while IFS= read -r -d '' f; do
  crd_files+=("$f")
done < <(find "${CRDS_DIR}" -maxdepth 1 -type f -name '*.yaml' -print0 | LC_ALL=C sort -z)

if [[ "${#crd_files[@]}" -eq 0 ]]; then
  echo "ERROR: no CRD fixtures found under ${CRDS_DIR}" >&2
  exit 2
fi

bytes_of() {
  wc -c < "$1" | tr -d ' '
}

say_header() {
  echo ""
  echo "Mini-Kubara Case 02: Large CRDs preflight (read-only)"
  echo "====================================================="
  echo ""
  echo "Annotation limit: ${ANNOTATION_LIMIT} bytes"
  echo "Fixtures root:    ${FIXTURES_DIR}"
  echo "CRDs dir:         ${CRDS_DIR}"
  echo "CRD fixtures:     ${#crd_files[@]}"
  echo ""
}

say_header

total_crds="${#crd_files[@]}"
risky_crds=0
ok_crds=0
widest=0
widest_name=""

printf '%-10s %-40s %12s %s\n' LANE FIXTURE BYTES RESULT
printf '%-10s %-40s %12s %s\n' ---- ------- ----- ------
for f in "${crd_files[@]}"; do
  name="$(basename "$f")"
  bytes=$(bytes_of "$f")
  if (( bytes > ANNOTATION_LIMIT )); then
    printf '%-10s %-40s %12s %s\n' "CRD-READ" "$name" "$bytes" "BLOCK client-side apply"
    risky_crds=$((risky_crds + 1))
  else
    printf '%-10s %-40s %12s %s\n' "CRD-READ" "$name" "$bytes" "OK under client-side apply"
    ok_crds=$((ok_crds + 1))
  fi
  if (( bytes > widest )); then
    widest=$bytes
    widest_name=$name
  fi
done

echo ""
echo "Summary"
echo "-------"
echo "Total CRDs:               ${total_crds}"
echo "Within annotation limit:  ${ok_crds}"
echo "Over annotation limit:    ${risky_crds}"
echo "Largest CRD body:         ${widest_name} (${widest} bytes)"
echo ""

appset_server_side="unknown"
appset_csa_migration_disabled="unknown"
appset_syncpolicy_location="unknown"
if [[ -f "${APPSET_FILE}" ]]; then
  if grep -q '^      syncPolicy:' "${APPSET_FILE}"; then
    appset_syncpolicy_location="template.spec"
  elif grep -q 'syncPolicy:' "${APPSET_FILE}"; then
    appset_syncpolicy_location="misnested"
  else
    appset_syncpolicy_location="missing"
  fi
  if grep -q 'ServerSideApply=true' "${APPSET_FILE}"; then
    appset_server_side="yes"
  else
    appset_server_side="no"
  fi
  if grep -q 'ClientSideApplyMigration=false' "${APPSET_FILE}"; then
    appset_csa_migration_disabled="yes"
  else
    appset_csa_migration_disabled="no"
  fi
fi

largest_crd_resource_ssa="unknown"
if [[ -n "${widest_name}" && -f "${CRDS_DIR}/${widest_name}" ]]; then
  if grep -q 'argocd.argoproj.io/sync-options:.*ServerSideApply=true' "${CRDS_DIR}/${widest_name}"; then
    largest_crd_resource_ssa="yes"
  else
    largest_crd_resource_ssa="no"
  fi
fi

echo "ApplicationSet:           ${APPSET_FILE#${FIXTURES_DIR}/}"
echo "Application syncPolicy:   ${appset_syncpolicy_location}"
echo "Argo ServerSideApply:     ${appset_server_side}"
echo "Argo CSA migration off:   ${appset_csa_migration_disabled}"
echo "Largest CRD resource SSA: ${largest_crd_resource_ssa}"
echo "Controller fixture:       ${CONTROLLER_FILE#${FIXTURES_DIR}/}"
echo "Dependent custom resource: ${WIDGET_FILE#${FIXTURES_DIR}/} (held back for Gate B)"
echo ""

if (( risky_crds > 0 )); then
  cat <<EOF
CONFIGHUB SAYS: ROUTE
Lane: CH-WRITE for the ApplicationSet, SETUP for any direct CRD apply
Scope: Mini-Kubara Case 02 large-CRD provider only
Wrong move: retrying client-side apply after a 262144-byte rejection; or
  calling a direct CRD workaround governed workload proof

Selected apply mode:
  - CRDs: server-side apply (kubectl apply --server-side=true --force-conflicts)
  - ApplicationSet syncPolicy location: ${appset_syncpolicy_location}
  - ApplicationSet syncOptions: ServerSideApply=true (already set: ${appset_server_side})
  - ApplicationSet syncOptions: ClientSideApplyMigration=false (set: ${appset_csa_migration_disabled})
  - oversized CRD per-resource sync-options annotation present: ${largest_crd_resource_ssa}
  - never stamp kubectl.kubernetes.io/last-applied-configuration on
    ${widest_name}

Why client-side apply is unsafe for ${widest_name}:
  The CRD body is ${widest} bytes, which is larger than the
  262144-byte annotation limit on its own. kubectl client-side apply
  stamps the full manifest into
  metadata.annotations.kubectl.kubernetes.io/last-applied-configuration,
  and the API server will reject the CRD with
  "metadata.annotations: Too long: may not be more than 262144 bytes".

Recommended delivery path for the future live run:
  - ApplicationSet sync path: Argo reconciles the workloads with
    ServerSideApply=true and ClientSideApplyMigration=false, which the fixture
    encodes;
  - the oversized CRD also carries the resource-level Argo
    ServerSideApply=true annotation, because the 2026-04-23 live pass showed
    Application-level SSA alone was not enough for this fixture;
  - if ConfigHub applies the CRD directly as setup, use a server-side
    apply worker or a bounded SETUP lane; label the action setup, not
    governed workload proof;
  - a direct kubectl apply must be --server-side=true --force-conflicts
    and must be recorded as SETUP/RECOVERY, not as the governed write
    path.

STOP. This is a read-only preflight. Do not create a cluster, install
Argo, apply CRDs, apply the dependent custom resource, or make any
ConfigHub write without an explicit Gate A approval.

Next gate (needs explicit Y/N):
  Gate A: apply the large-CRD provider path using server-side apply and
          prove both CRDs exist without annotation-size rejection.
EOF
else
  cat <<EOF
CONFIGHUB SAYS: ROUTE
Lane: CH-WRITE for the ApplicationSet
Scope: Mini-Kubara Case 02 large-CRD provider only
Wrong move: assuming the annotation-size risk is hypothetical

No fixture CRD currently exceeds ${ANNOTATION_LIMIT} bytes. The case still
exists to rehearse the delivery-mode decision. Real public CRDs such as
external-secrets and cert-manager are known to cross the limit, so keep
ServerSideApply=true on the ApplicationSet (already set: ${appset_server_side})
and do not fall back to client-side apply.

STOP. This is a read-only preflight. Do not create a cluster, install
Argo, apply CRDs, apply the dependent custom resource, or make any
ConfigHub write without an explicit Gate A approval.
EOF
fi

exit 0
