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
The two safe paths are:

  - server-side apply: kubectl apply --server-side=true --force-conflicts,
    which does not stamp last-applied-configuration at all;
  - an Argo/Flux/ConfigHub sync option that forwards ServerSideApply=true
    to the controller (the ApplicationSet fixture already does this).

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
if [[ -f "${APPSET_FILE}" ]]; then
  if grep -q 'ServerSideApply=true' "${APPSET_FILE}"; then
    appset_server_side="yes"
  else
    appset_server_side="no"
  fi
fi

echo "ApplicationSet:           ${APPSET_FILE#${FIXTURES_DIR}/}"
echo "Argo ServerSideApply:     ${appset_server_side}"
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
  - ApplicationSet syncOptions: ServerSideApply=true (already set: ${appset_server_side})
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
    ServerSideApply=true, which the fixture already encodes;
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
