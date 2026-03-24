# AI Start Here

Use this page when you want to drive the `bundle-evidence-sample` safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. say what the GUI shows today
6. say what the GUI does not show yet
7. name the GUI feature ask and cite the issue number if one exists; if not, say that explicitly
8. tell the human to open the GUI and give them time to inspect it
9. ask `Ready to continue?`
10. do not move on until the human says to continue

## Suggested Prompt

```text
Read incubator/global-app-layer/bundle-evidence-sample/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## Stage 1: Preview The Sample (read-only)

```bash
cd incubator/global-app-layer/bundle-evidence-sample
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no in-product preview for the bundle evidence sample before local output is generated
- GUI feature ask: no issue filed yet for a package-level preview panel for bundle evidence examples

Pause after this stage.

## Stage 2: Generate The Bundle Evidence View (read-only with respect to ConfigHub and live infrastructure)

```bash
./setup.sh
```

What you should see after:

- five JSON evidence files in `sample-output/`
- one HTML summary page in `sample-output/bundle-evidence.html`

GUI checkpoint:

- GUI now: open `sample-output/bundle-evidence.html`
- GUI gap: this is only a local artifact, not a real ConfigHub bundle screen yet
- GUI feature ask: a first-class bundle evidence page in ConfigHub with the same sections

Pause after this stage.

## Stage 3: Verify The Evidence (read-only)

```bash
./verify.sh
jq '{bundle, target, deploymentVariant}' sample-output/bundle-record.json
jq '{checksums, verification}' sample-output/bundle-integrity.json
jq '{sbom, attestations}' sample-output/bundle-supply-chain.json
jq '{deployer, liveEvidence}' sample-output/bundle-handoff.json
```

Use the evidence like this:

- `bundle-record.json` proves publication facts and target-aware ownership
- `bundle-contents.json` proves per-component artifact structure
- `bundle-integrity.json` proves checksum coverage and verification result
- `bundle-supply-chain.json` proves SBOM and attestation references exist
- `bundle-handoff.json` proves a downstream deployer consumed the same bundle digest

GUI checkpoint:

- GUI now: `sample-output/bundle-evidence.html` shows all four evidence layers together
- GUI gap: there is no linked in-product drill-down from bundle facts to recipe provenance and live evidence yet
- GUI feature ask: a linked bundle, integrity, and handoff inspector in ConfigHub

Pause after this stage.

## Stage 4: Cleanup

```bash
./cleanup.sh
```

This removes only local sample output.
