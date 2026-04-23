#!/usr/bin/env bash
# Regenerate fixtures/workloads/crds/stores-large.yaml.
#
# The file is intentionally large so that preflight tooling can detect an
# annotation-size risk before kubectl client-side apply stamps the full
# manifest into metadata.annotations["kubectl.kubernetes.io/last-applied-configuration"].
#
# The output is deterministic given the constants below and a fixed seed.
# Commit both this generator and the generated YAML so the preflight signal
# is repeatable without regeneration.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="${SCRIPT_DIR}/workloads/crds/stores-large.yaml"

FIELD_COUNT="${FIELD_COUNT:-500}"

LOREM="Stores a platform resource used by the mini-kubara case-02 large-CRD fixture. This description is deliberately verbose so that the composed CRD body, when JSON-encoded by kubectl client-side apply and stamped into the last-applied-configuration annotation, exceeds the Kubernetes 262144-byte metadata.annotations limit. The fixture lets preflight tooling detect the risk before any live apply, and forces the delivery mode decision to become explicit."

mkdir -p "$(dirname "$OUT")"

{
  cat <<'HEADER'
# GENERATED FILE - do not edit by hand. Rerun generate-stores-large.sh.
#
# Intentionally large CRD fixture for Mini-Kubara Case 02.
# Serialized body exceeds the Kubernetes 262144-byte annotation limit so
# that client-side kubectl apply will be rejected with
#   metadata.annotations: Too long: may not be more than 262144 bytes
# The case-02 preflight uses this to force an explicit apply-mode decision.
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: stores.platform.example.com
  labels:
    mini-kubara: case-02
    mini-kubara.confighub.example.com/risk: large-annotation
  annotations:
    mini-kubara.confighub.example.com/purpose: "Intentionally large CRD fixture for annotation-size preflight"
spec:
  group: platform.example.com
  names:
    kind: Store
    listKind: StoreList
    plural: stores
    singular: store
    shortNames:
      - st
  scope: Namespaced
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          properties:
            spec:
              type: object
              properties:
HEADER

  for i in $(seq 0 $((FIELD_COUNT - 1))); do
    idx=$(printf '%03d' "$i")
    printf '                field%s:\n' "$idx"
    printf '                  type: string\n'
    printf '                  description: "%s Field index %s carries descriptive metadata for the preflight test."\n' \
      "$LOREM" "$idx"
  done

  cat <<'FOOTER'
            status:
              type: object
              properties:
                phase:
                  type: string
                  description: "Observed phase of the Store object."
      subresources:
        status: {}
      additionalPrinterColumns:
        - name: Phase
          type: string
          jsonPath: .status.phase
FOOTER
} > "$OUT"

bytes=$(wc -c < "$OUT" | tr -d ' ')
echo "Generated $OUT (${bytes} bytes)"
