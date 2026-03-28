# Bundle Publication Walkthrough

This page is the concrete walkthrough that sits between the AICR fit docs and the open TODO list.

It answers one practical question:

- if a layered recipe eventually produces a deployable bundle, what should a believable ConfigHub demo show before, during, and after that publication step?

This is an honest walkthrough, not a claim that the whole productized bundle story already exists.

## What This Walkthrough Is For

Use this when you want to explain the bundle story in stages:

1. where the bundle comes from
2. what facts should be visible at publication time
3. what integrity and attestation evidence should be attached
4. how the bundle connects back to recipe provenance and later deployment

## The Important Boundary

Today the `global-app-layer` package proves the recipe side much more strongly than the bundle-publication side.

It already proves:

- layered recipe inputs
- deployment variants at the leaf
- target binding and delivery-path choice
- recipe provenance across clone chains and recipe manifests
- **Flux OCI** as a working controller-oriented bundle delivery path

It does not yet fully prove:

- end-to-end bundle publication as a first-class in-product flow
- bundle inspection in ConfigHub
- attached SBOM, checksum, or attestation inspection in ConfigHub
- **Argo OCI** bundle delivery (target-state, not yet implemented)

## Delivery Matrix For Bundle Publication

| Delivery Mode | Status | Bundle Story |
|---------------|--------|--------------|
| **Direct Kubernetes** | Fully working | Worker applies, no OCI bundle |
| **Flux OCI** | Current standard | OCI bundle published, Flux reconciles |
| **Argo OCI** | Target-state, not implemented | OCI bundle published, Argo reconciles |
| **ArgoCDRenderer** | Working, limited scope | Renderer path only, not bundle delivery |

**Flux OCI** is the current standard for the bundle publication story. It proves:

- OCI artifact published to registry
- Bundle digest recorded
- Flux consumes the exact digest
- Controller manages workload lifecycle

**ArgoCDRenderer** is not the same as Argo OCI bundle delivery. It is a renderer path that expects Argo `Application` payloads and does not publish OCI bundles.

So the right way to use this page is:

- show what is already real (recipe provenance, Flux OCI)
- show what the next bundle product surface should look like
- be explicit about what is still a gap (Argo OCI, in-product bundle inspection)

## Stage 1: Inspect The Recipe Chain

Start from one worked example such as:

- [`realistic-app`](./realistic-app/README.md)
- [`gpu-eks-h100-training`](./gpu-eks-h100-training/README.md)

Read-only first:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

Then materialize the recipe objects:

```bash
./setup.sh
./verify.sh
```

What this proves now:

- the recipe is stored as real ConfigHub units
- each layer has concrete provenance
- the deployment variant exists as a real unit at the leaf

GUI now:

- ConfigHub can already show spaces, units, trees, manifests, diffs, and recipe provenance

GUI gap:

- there is no first-class bundle publication panel yet

GUI feature ask:

- a bundle tab or artifact tab on the deployment variant and recipe manifest

Pause here in a demo and make sure the human understands that the recipe chain is the source of the later bundle.

## Stage 2: Identify The Deployment Variant And Target

Use the leaf deployment unit plus target binding as the transition point.

Examples:

```bash
cd incubator/global-app-layer/realistic-app
./set-target.sh <space/target>
./verify.sh
```

What this proves now:

- the bundle belongs to a deployment path, not to the abstract recipe alone
- target choice is explicit
- direct apply and delegated GitOps delivery are different leaf outcomes

GUI now:

- ConfigHub can show the bound deployment unit and the chosen target relationship

GUI gap:

- it does not yet present a clear “this is the bundle-producing edge” story

GUI feature ask:

- a target-aware publish/delivery panel that makes the output artifact type explicit

## Stage 3: Publish Or Record The Bundle

This is the key missing productized stage.

In AICR terms, this is where a validated recipe becomes deployable artifacts.

For a believable ConfigHub story, the publication moment should record:

- bundle URI or OCI reference
- bundle digest
- publication time
- deployer type
- deployment variant that produced it
- target that produced it
- exact recipe manifest and unit revisions used

What is real today:

- the package already talks about bundle hints and bundle digests when known
- the target and deployment variant side of the story is already modeled

What is not yet first-class today:

- a durable in-product bundle publication record and inspection flow

GUI now:

- no dedicated bundle publication surface yet

GUI gap:

- no clear "publish bundle" result page with artifact facts

GUI feature ask:

- a first-class bundle record linked from the deployment variant and target

## Stage 4: Inspect Bundle Contents

For an AICR-shaped walkthrough, this is where the operator asks:

- what did we actually publish?
- which components are inside?
- what deployer should consume it?

The bundle inspection view should be able to show:

- per-component artifact list
- charts or deployer payload references
- values or manifest references
- generated README or deploy notes

What is real today:

- the examples explain bundle intent and bundle ownership conceptually
- the package does not yet have a dedicated bundle inspector

GUI now:

- no bundle browser yet

GUI gap:

- no side-by-side view of recipe provenance and bundle contents

GUI feature ask:

- a bundle contents browser linked back to the recipe chain

## Stage 5: Inspect Integrity, SBOMs, And Attestations

This is where the supply-chain story becomes useful instead of abstract.

The important distinction is:

- bundle contents tell you what is deployable
- checksums, SBOMs, and attestations tell you whether that deployable artifact is trustworthy and what it contains

The inspection view should show:

- checksum manifest and verification result
- SBOM reference or digest
- attestation reference or digest
- provenance statement reference
- image digests covered by those records

What is real today:

- the `global-app-layer` docs now explain these concepts
- the package does not yet provide a first-class example surface for them

GUI now:

- no first-class integrity or attestation view in this package

GUI gap:

- no clear place to answer “is this the exact bundle we expected, and what evidence proves that?”

GUI feature ask:

- a supply-chain evidence panel attached to the bundle record

## Stage 6: Hand Off To The Deployer

A believable AICR-shaped ConfigHub story should then show what consumed the artifact.

That means showing one of:

| Delivery Mode | What Gets Published | What Consumes It |
|---------------|---------------------|------------------|
| Direct Kubernetes | Nothing (worker applies directly) | Cluster via worker |
| **Flux OCI** (current standard) | OCI artifact | Flux OCIRepository + Kustomization |
| **Argo OCI** (target-state) | OCI artifact | Argo Application pointing at OCI source |
| ArgoCDRenderer | Argo Application CRD | ArgoCD renderer API (not workload delivery) |

**Flux OCI** is the current standard for the bundle handoff story because it proves the complete flow: OCI publication, controller consumption, workload delivery.

**Argo OCI** is the target-state direction but is not yet implemented.

**ArgoCDRenderer** does not publish bundles — it sends Application CRDs to ArgoCD for rendering. It is a companion path, not the bundle delivery standard.

The important point is not that ConfigHub must be the only deployer.
The important point is that the published bundle and the chosen deployment path stay connected.

What should be visible:

- which deployer consumed the artifact
- when it consumed it
- what digest or version it consumed
- what later cluster evidence matches that handoff

## Stage 7: Compare Bundle Evidence With Live Evidence

This is where ConfigHub can become more useful than a plain artifact store.

The final operator question is not only:

- what bundle was published?

It is also:

- is the cluster running what that bundle says it should run?

That comparison should connect:

- recipe provenance
- bundle facts
- integrity evidence
- deployer handoff
- live cluster evidence

This is the stronger long-term operational story.

## What This Walkthrough Proves Today

Today this walkthrough can honestly prove:

- the recipe provenance side
- the deployment-variant side
- the conceptual role of bundle ownership by target
- the exact gaps in the current product story

## What This Walkthrough Does Not Yet Prove

It does not yet prove, end to end:

- first-class bundle publication in ConfigHub
- first-class bundle browsing in ConfigHub
- first-class SBOM or attestation inspection in ConfigHub
- full deployer-consumed-artifact lineage in one screen

## How To Use This In A Demo

Use this walkthrough after one of the recipe demos, not before it.

Best order:

1. show the recipe chain with `realistic-app` or `gpu-eks-h100-training`
2. show the deployment variant and target
3. explain what the bundle publication record should contain
4. explain what integrity and attestation evidence should add
5. explain how this would later connect to live cluster evidence

That keeps the demo honest and still shows the product direction clearly.

## Related Pages

- [01-nvidia-aicr-fit.md](./01-nvidia-aicr-fit.md)
- [02-recipes-and-layers-spec.md](./02-recipes-and-layers-spec.md)
- [04-bundles-attestation-and-todo.md](./04-bundles-attestation-and-todo.md)
- [bundle-evidence-sample/README.md](./bundle-evidence-sample/README.md)
- [06-bundle-evidence-gui-spec.md](./06-bundle-evidence-gui-spec.md)
- [confighub-aicr-value-add.md](./confighub-aicr-value-add.md)
- [whole-journey.md](./whole-journey.md)
