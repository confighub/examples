# Bundle Evidence Sample

This example is a small fixture-backed companion to the `global-app-layer` package.

It turns the AICR bundle story into a partly runnable path:

- one published bundle record
- one bundle contents record
- one integrity record
- one supply-chain record
- one deployer handoff record
- one local HTML summary page generated from those records

## What This Example Is For

Use this when you want a concrete bundle walkthrough without pretending the full in-product bundle publication flow already exists.

It does not mutate ConfigHub and does not require a cluster.

## What It Reads

It reads:

- the copied bundle-evidence fixtures under `fixtures/published-bundle/`
- `jq`

## What It Writes

It writes local output under `sample-output/`:

- `bundle-record.json`
- `bundle-contents.json`
- `bundle-integrity.json`
- `bundle-supply-chain.json`
- `bundle-handoff.json`
- `bundle-evidence.html`

It does not write ConfigHub state and does not mutate live infrastructure.

## Read-Only First

```bash
cd incubator/global-app-layer/bundle-evidence-sample
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Quick Start

```bash
./setup.sh
./verify.sh
```

## What Success Looks Like

You should see:

- one bundle URI and digest
- one deployment variant and target binding recorded in the publication record
- per-component bundle contents for `gpu-operator` and `nvidia-device-plugin`
- checksum verification marked successful
- SBOM and attestation references present
- one Flux handoff record consuming the same bundle digest
- one local HTML summary page for GUI-style inspection

## Evidence To Check

```bash
./verify.sh
jq '{bundle, target, deploymentVariant}' sample-output/bundle-record.json
jq '{componentCount: (.components | length), deployerPayloads}' sample-output/bundle-contents.json
jq '{checksums, verification}' sample-output/bundle-integrity.json
jq '{sbom, attestations, signatureVerification}' sample-output/bundle-supply-chain.json
jq '{deployer, liveEvidence}' sample-output/bundle-handoff.json
```

## Why This Example Matters

This is the smallest honest bridge between:

- layered recipe provenance
- bundle publication facts
- integrity and supply-chain evidence
- downstream deployer handoff

It is a sample, not a full productized publisher. That is intentional.

## AI-Safe Path

Start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)
- [prompts.md](./prompts.md)
- [../06-bundle-evidence-gui-spec.md](../06-bundle-evidence-gui-spec.md)

## Cleanup

```bash
./cleanup.sh
```
