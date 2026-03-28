# AICR Bundles, Attestation, and TODO for `global-app-layer`

This note extends the old status-and-gaps page with one missing topic: what NVIDIA AICR means by bundles, integrity, SBOMs, and attestations, and what ConfigHub should eventually show or preserve.

Primary reference points:

- [NVIDIA blog: Validate Kubernetes for GPU Infrastructure with Layered, Reproducible Recipes](https://developer.nvidia.com/blog/validate-kubernetes-for-gpu-infrastructure-with-layered-reproducible-recipes/)
- [NVIDIA AICR repo](https://github.com/NVIDIA/aicr)

## Current Status

The package is now a coherent teaching and demo set for layered recipes in ConfigHub.

It has:

- a minimal ConfigHub first step: [`00-config-hub-hello-world.md`](./00-config-hub-hello-world.md)
- a simple one-component proof: [`single-component`](./single-component/)
- a small multi-component app proof: [`frontend-postgres`](./frontend-postgres/)
- a more realistic app proof: [`realistic-app`](./realistic-app/)
- a domain-shaped GPU recipe proof: [`gpu-eks-h100-training`](./gpu-eks-h100-training/)
- a practical explanation of ConfigHub's value on top of AICR: [confighub-aicr-value-add.md](./confighub-aicr-value-add.md)
- a package-owned e2e layer: [e2e/](./e2e/)

That is enough to teach the core model and run believable demos.

## What Is Working Well

### 1. The recipe model is clear

The examples consistently show:

- ordered variant chains, implemented with clone links
- explicit layer meaning
- explicit recipe manifests for provenance
- deployment layers separated from shared recipe layers

### 2. Upgrades are believable

The package now gives a credible story for:

- shared updates
- downstream propagation
- preserved deployment-local values

### 3. Delivery is no longer hand-wavy

The package now has one place for:

- direct Kubernetes delivery (fully working)
- Flux OCI delivery (current standard controller-oriented bundle path)
- Argo OCI delivery (target-state, not yet implemented)
- ArgoCDRenderer (renderer path, not workload delivery)
- brownfield import
- bridge flows between import and layering

The key distinction: **Flux OCI** is the current standard for controller-oriented bundle delivery. **Argo OCI** is the target-state direction but is not yet implemented. **ArgoCDRenderer** is a renderer path that expects Argo `Application` payloads — it is not the same as OCI bundle delivery.

### 4. The package is easier to explain

The combination of:

- [README.md](./README.md)
- [how-it-works.md](./how-it-works.md)
- [confighub-aicr-value-add.md](./confighub-aicr-value-add.md)

is now much stronger than the older scattered explanation.

## What NVIDIA AICR Means By A Bundle

Based on the current NVIDIA material, there are two closely related but different stories.

### 1. Generated deployment bundles

The `aicr bundle` step materializes a validated recipe into deployable artifacts.

From the current AICR repo description, the output is a `bundles/` directory containing per-component deployment material such as:

- one folder per component
- Helm charts or deployer-ready files
- values files
- checksums
- README or deployer guidance
- deployer configs for tools such as Helm or Argo CD

This is the deployment handoff artifact.

### 2. Supply-chain security artifacts

Separately, NVIDIA describes release-level supply-chain security artifacts such as:

- SLSA provenance
- signed SBOMs
- image attestations, for example with cosign
- checksum verification

This is not the same thing as the rendered deployment bundle itself.

It is the evidence that the tooling or published artifacts came from a trustworthy build and can be verified.

That distinction matters.

A good ConfigHub story should not collapse these into one vague word.

## What Integrity, SBOMs, and Attestations Mean In Practice

### Checksums

A checksum is the quickest integrity signal.

It answers:

- is this file exactly the one that was published?
- did it change in transit or at rest?

For bundles, checksums usually belong on generated files or archives.

### SBOMs

An SBOM is a software bill of materials.

It answers:

- what packages, images, or components are inside this artifact?
- what exact versions are present?

For an AICR-shaped flow, the useful question is not just "what recipe did I ask for?" but also "what concrete software ended up inside the published deployable artifact?"

### Attestations

An attestation is a signed claim about an artifact.

It answers questions such as:

- what workflow produced this?
- what source inputs were used?
- what checks ran?
- what image digest or artifact digest does this claim apply to?

Image attestations are the OCI-specific version of that idea: a signed statement attached to an image digest or related artifact.

### Provenance

Provenance is the broader chain of custody story.

It connects:

- source recipe inputs
- render or bundle step
- produced artifacts
- validation results
- signatures or attestations

ConfigHub already has part of this story at the config level through unit revisions, hashes, clone ancestry, and recipe manifests.

The missing part is making bundle publication evidence equally visible.

## What ConfigHub Can Already Model Well

Today ConfigHub can already model:

- the layered recipe inputs
- the explicit chain of specialization
- the deployment variant that leads to a target
- revision history and provenance of config objects
- bundle hints or bundle digests when known
- multiple deploy paths such as direct apply or delegated GitOps delivery

That is enough to explain the recipe side of AICR convincingly.

## What ConfigHub Does Not Yet Show As A First-Class Bundle Story

The package does not yet present a strong end-to-end walkthrough for:

- generate bundle
- inspect bundle contents
- inspect bundle checksums
- inspect SBOM and attestation links or digests
- compare bundle evidence with recipe provenance
- show what later deployers consumed

That is the specific gap behind the earlier vague phrase "full bundle publication as a first-class story."

## What ConfigHub Should Show Or Preserve Later

A future ConfigHub bundle story should make these visible.

### Bundle publication facts

- bundle URI or OCI reference
- bundle digest
- publication time
- target that produced it
- deployment variant that produced it
- exact recipe manifest or unit revisions used

### Bundle contents facts

- per-component artifact list
- deployer type, for example Helm, Argo CD, or Flux handoff
- values or manifest payload references
- README or generated deployment notes when present

### Integrity facts

- checksum manifest
- verification result
- which files or artifacts the checksum set covers

### Supply-chain evidence facts

- SBOM reference or digest
- attestation reference or digest
- provenance statement reference
- signature verification result
- image digests that the attestations apply to

### Validation facts

- snapshot reference used for validation
- readiness or compatibility result
- post-deploy or conformance report reference when available

## What ConfigHub Probably Should Not Do

ConfigHub does not need to become a full replacement for artifact registries, SBOM stores, or signing systems.

A better role is:

- preserve references and digests
- show verification status
- connect bundle evidence back to recipe provenance
- make review and approval easier

That is more plausible and more useful than trying to duplicate every external artifact system.

## Open TODOs For This Package

### 1. Turn the new bundle walkthrough and sample into a stronger runnable path

The package now has:

- [05-bundle-publication-walkthrough.md](./05-bundle-publication-walkthrough.md)
- [bundle-evidence-sample/README.md](./bundle-evidence-sample/README.md)

That is enough to explain the model and to demo local bundle publication evidence honestly.

The next step is to turn more of that into a runnable end-to-end path that shows:

- recipe chain
- bundle output
- bundle inspection
- integrity evidence
- downstream deploy handoff
- optional live evidence against a real deployer

### 2. Turn the GUI bundle evidence spec into a concrete product slice

The package now has:

- [06-bundle-evidence-gui-spec.md](./06-bundle-evidence-gui-spec.md)

The next step is to turn that into a concrete product or design slice that says plainly:

- generated bundle contents
- checksum role
- SBOM role
- attestation role
- what is proven vs not proven

and makes those visible in one place instead of across several documents.

### 3. Keep the GUI-first bundle page grounded in the sample and walkthrough

The GUI page should continue to show:

- recipe provenance on one side
- bundle facts on the other
- verification state and missing evidence clearly marked

### 4. Keep preflight and live-target clarity improving

The examples still assume the user can sort out:

- authentication
- which worker is active
- which target to use
- whether a cluster is fresh and connected

### 5. Add a phased operational story

The examples still do not yet show a formal lifecycle such as:

- snapshot
- readiness validate
- deploy
- post-deploy health
- conformance

That remains the strongest reason to keep exploring a better `cub-proc` or `Operation` story.

### 6. Add a larger fleet-style example

The package now demonstrates preserved local overrides, but it still lacks a fuller fleet example where many similar deployments differ by one or two meaningful operational parameters.

## Bottom Line

The package is already good enough to teach layered recipes and the basic AICR fit.

What remains is not the recipe model.
What remains is the bundle and evidence product story:

- clearer bundle publication
- clearer integrity and attestation explanation
- clearer GUI surfaces for bundle evidence
- clearer operational sequencing
