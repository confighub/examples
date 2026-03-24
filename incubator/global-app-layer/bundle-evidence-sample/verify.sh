#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"

if [[ ! -f "$OUTPUT_DIR/bundle-record.json" ]]; then
  "$SCRIPT_DIR/setup.sh" >/dev/null
fi

jq -e '.bundle.uri == "oci://ghcr.io/confighub/examples/gpu-eks-h100-training/cluster-a"' "$OUTPUT_DIR/bundle-record.json" >/dev/null
jq -e '.target.kind == "FluxOCIWriter" and .deploymentVariant.unit == "gpu-training-cluster-a-flux"' "$OUTPUT_DIR/bundle-record.json" >/dev/null
jq -e '.components | length == 2' "$OUTPUT_DIR/bundle-contents.json" >/dev/null
jq -e '.checksums.algorithm == "sha256" and .verification.verified == true' "$OUTPUT_DIR/bundle-integrity.json" >/dev/null
jq -e '.sbom.reference and (.attestations | length >= 2) and .signatureVerification.verified == true' "$OUTPUT_DIR/bundle-supply-chain.json" >/dev/null
jq -e '.deployer.kind == "Flux" and (.deployer.consumedReference | contains("sha256"))' "$OUTPUT_DIR/bundle-handoff.json" >/dev/null
jq -e '.liveEvidence.cluster == "cluster-a" and (.liveEvidence.objects | length >= 2)' "$OUTPUT_DIR/bundle-handoff.json" >/dev/null
test -f "$OUTPUT_DIR/bundle-evidence.html"

echo "Bundle evidence sample checks passed"
