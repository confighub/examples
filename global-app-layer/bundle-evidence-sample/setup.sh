#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIXTURE_DIR="$SCRIPT_DIR/fixtures/published-bundle"
OUTPUT_DIR="$SCRIPT_DIR/sample-output"
EXPLAIN=0
EXPLAIN_JSON=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --explain) EXPLAIN=1 ;;
    --explain-json) EXPLAIN_JSON=1 ;;
    *) echo "Unknown argument: $1" >&2; exit 1 ;;
  esac
  shift
done

if [[ $EXPLAIN -eq 1 ]]; then
  cat <<TEXT
This example will:
- read fixtures/published-bundle as a bundle evidence sample
- copy bundle publication, contents, integrity, supply-chain, and handoff JSON into sample-output/
- generate one local HTML summary page from those JSON files
- not write ConfigHub state
- not mutate live infrastructure
TEXT
  exit 0
fi

if [[ $EXPLAIN_JSON -eq 1 ]]; then
  jq -n '{example:"bundle-evidence-sample", mutatesConfighub:false, mutatesLiveInfrastructure:false, source:"fixtures/published-bundle"}'
  exit 0
fi

mkdir -p "$OUTPUT_DIR"
cp "$FIXTURE_DIR"/*.json "$OUTPUT_DIR/"

bundle_uri=$(jq -r '.bundle.uri' "$OUTPUT_DIR/bundle-record.json")
bundle_digest=$(jq -r '.bundle.digest' "$OUTPUT_DIR/bundle-record.json")
target_name=$(jq -r '.target.name' "$OUTPUT_DIR/bundle-record.json")
deployment_variant=$(jq -r '.deploymentVariant.unit' "$OUTPUT_DIR/bundle-record.json")
component_count=$(jq -r '.components | length' "$OUTPUT_DIR/bundle-contents.json")
sbom_ref=$(jq -r '.sbom.reference' "$OUTPUT_DIR/bundle-supply-chain.json")
attestation_ref=$(jq -r '.attestations[0].reference' "$OUTPUT_DIR/bundle-supply-chain.json")
deployer_kind=$(jq -r '.deployer.kind' "$OUTPUT_DIR/bundle-handoff.json")
consumed_ref=$(jq -r '.deployer.consumedReference' "$OUTPUT_DIR/bundle-handoff.json")

cat > "$OUTPUT_DIR/bundle-evidence.html" <<HTML
<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Bundle Evidence Sample</title>
  <style>
    body { font-family: Georgia, serif; margin: 2rem; background: #f8f7f3; color: #1f2a37; }
    h1, h2 { margin-bottom: 0.4rem; }
    .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1rem; }
    .card { background: white; border: 1px solid #d5d0c5; border-radius: 12px; padding: 1rem; box-shadow: 0 8px 24px rgba(0,0,0,0.05); }
    code { background: #f0ece3; padding: 0.1rem 0.3rem; border-radius: 4px; }
    ul { padding-left: 1.2rem; }
  </style>
</head>
<body>
  <h1>Bundle Evidence Sample</h1>
  <p>This local page is a GUI-style summary of the fixture-backed bundle publication records.</p>
  <div class="grid">
    <section class="card">
      <h2>Publication</h2>
      <ul>
        <li>Bundle URI: <code>${bundle_uri}</code></li>
        <li>Bundle digest: <code>${bundle_digest}</code></li>
        <li>Target: <code>${target_name}</code></li>
        <li>Deployment variant: <code>${deployment_variant}</code></li>
      </ul>
    </section>
    <section class="card">
      <h2>Contents</h2>
      <ul>
        <li>Component count: <code>${component_count}</code></li>
        <li>Source file: <code>bundle-contents.json</code></li>
      </ul>
    </section>
    <section class="card">
      <h2>Supply Chain</h2>
      <ul>
        <li>SBOM ref: <code>${sbom_ref}</code></li>
        <li>Attestation ref: <code>${attestation_ref}</code></li>
      </ul>
    </section>
    <section class="card">
      <h2>Handoff</h2>
      <ul>
        <li>Deployer: <code>${deployer_kind}</code></li>
        <li>Consumed ref: <code>${consumed_ref}</code></li>
      </ul>
    </section>
  </div>
</body>
</html>
HTML

echo "Saved bundle evidence outputs to: $OUTPUT_DIR"
