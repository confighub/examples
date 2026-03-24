# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- proves:
  - the example is fixture-backed and no-cluster
  - it does not mutate ConfigHub
  - it writes only local sample-output artifacts
- expected anchors:
  - `.example == "bundle-evidence-sample"`
  - `.mutatesConfighub == false`
  - `.mutatesLiveInfrastructure == false`
  - `.source == "fixtures/published-bundle"`

## Publication Contract

### `jq '{bundle, target, deploymentVariant}' sample-output/bundle-record.json`

- mutates: no
- proves:
  - one concrete bundle digest is tied to one deployment variant and one target

## Integrity Contract

### `jq '{checksums, verification}' sample-output/bundle-integrity.json`

- mutates: no
- proves:
  - checksum coverage exists
  - verification succeeded for the published bundle

## Supply-Chain Contract

### `jq '{sbom, attestations, signatureVerification}' sample-output/bundle-supply-chain.json`

- mutates: no
- proves:
  - SBOM and attestation references are preserved
  - signature verification result is available

## Handoff Contract

### `jq '{deployer, liveEvidence}' sample-output/bundle-handoff.json`

- mutates: no
- proves:
  - the same bundle digest is connected to a downstream deployer handoff
  - later live evidence can be attached to that handoff
