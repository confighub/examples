# New-User Sense Check And Plan

**Date:** 2026-03-27
**Repo:** `/Users/alexis/Public/github-repos/examples`

For the OCI-standardization direction and delivery matrix decisions, also read:

- `incubator/planning/2026-03-28-oci-standard-aicr-bundles-and-today-plan.md`
- `incubator/WHY_CONFIGHUB.md` (new front door with glossary and delivery matrix)

## Lens

This pass uses the perspective of a brand-new ConfigHub user:

- I do not already know the object model
- I do not already know which examples are current
- I do not already know the difference between import, apply, evidence, layered recipes, workers, or GitOps renderer targets
- I want one fast reason to care before I will tolerate a taxonomy

## Short Verdict

The incubator examples make noticeably more sense than before, but the repo still feels like it was written by people who already know the map.

The good news:

- the standard Argo and Flux stories are now believable and runnable
- the incubator landing docs now explain more clearly why several major examples exist
- the newer docs are much more honest about real vs simulated vs import-only

The remaining problem:

- the repo still presents too many categories before it gives one simple mental model
- a new user can tell that there are many examples, but not yet why ConfigHub itself is worth learning before they start reading example families
- the NVIDIA-shaped examples are explained better than before, but they are still not planned as a first-class real end-to-end proof track
- the AICR bundle story is documented honestly, but it is still mostly a walkthrough plus a fixture-backed sample rather than a real end-to-end publication and inspection proof

## What Already Works For A New User

### 1. The standard Argo and Flux stories are now plausible front doors

- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`

These now have a clear first pass:

- healthy app first
- contrast second
- evidence and import story clear

This is the strongest current part of the incubator.

### 2. Reason-for-existing sections help a lot

The newer “What this example is for” sections remove a lot of guesswork, especially in:

- `incubator/platform-write-api`
- `incubator/springboot-platform-app`
- `incubator/global-app-layer/*`
- `incubator/cub-proc`

### 3. The repo is more honest than confusing

The reality taxonomy is valuable.

A new user may not understand all the terms yet, but the docs are usually honest about:

- what is real
- what is simulated
- what needs a cluster
- what touches ConfigHub

That trust matters.

## Main New-User Confusion Points

### 1. There are still too many front doors

A brand-new user sees:

- `START_HERE.md`
- `incubator/README.md`
- `incubator/AI_START_HERE.md`
- many example-specific `AI_START_HERE.md` files

That is manageable for experienced contributors, but still noisy for someone new.

### 2. The repo explains categories before it explains the product

New users need a one-paragraph answer to:

> Why would I use ConfigHub at all instead of just Git, Helm, Argo, or Flux?

Right now the examples often answer this indirectly, but the front door still tends to start with example families, not the core product reason.

### 3. The jargon arrives too early

Terms like these appear quickly:

- WET
- brownfield
- clone link
- variant
- renderer
- `fluxoci`
- deployment variant
- mutation plane

Most of these terms are useful, but too many appear before the user has one stable success story in mind.

### 4. “Choose by goal” is better now, but still broad

The new “Choose By Reason” sections are a real improvement.

But they still need one higher-level question above them:

> Am I here to see import, apply, model, or procedure?

Without that, the user is still choosing from a long list of examples instead of from a small set of product reasons.

### 5. Some examples are clearer than the package they live in

Several individual example READMEs are now clearer than the package-level landing docs around them.

That means a user often has to click into a subexample before the story becomes crisp.

### 6. Delivery modes are still not explained as one coherent matrix

The repo is much better than before, but a new user still has to infer too much about the difference between:

- direct Kubernetes apply
- Flux OCI delivery
- renderer-oriented paths such as `ArgoCDRenderer`
- future Argo OCI delivery

That matters because the product promise changes depending on which one is in use.

The front door should make this explicit early.

### 7. NVIDIA is still structurally clearer than it is operationally proven

`incubator/global-app-layer/gpu-eks-h100-training` now explains why it exists, but it is still mainly a model and structure story.

A new user can understand the NVIDIA-shaped chain better than before, but they still cannot point to one standard NVIDIA example and say:

> this is a fully real ConfigHub-to-deployment-to-verification proof on real GPU-capable infrastructure

That is a gap.

### 8. The AICR bundle story is easier to describe than to prove

The bundle material is useful:

- `incubator/global-app-layer/05-bundle-publication-walkthrough.md`
- `incubator/global-app-layer/bundle-evidence-sample/README.md`
- `incubator/global-app-layer/04-bundles-attestation-and-todo.md`

But a new user still cannot point to one standard bundle example and say:

> this is a real bundle publication, with real bundle facts, real integrity evidence, real supply-chain evidence, and a real downstream handoff tied to the same digest

Today the bundle story is best understood as:

- recipe side: real and better proven
- bundle side: explained honestly, partly runnable, still not a first-class real proof

That is another gap.

## New-User Mental Model We Should Optimize For

If the incubator is working, a new user should be able to understand ConfigHub like this:

1. `Import`: ConfigHub can ingest and organize live or GitOps-managed config
2. `Inspect`: ConfigHub can compare, scan, report, and explain what is there
3. `Mutate`: ConfigHub can act as the write API for operational config
4. `Apply`: ConfigHub can deliver real changes through real targets
5. `Model`: ConfigHub can represent layered or governed configuration structures
6. `Procedure`: some workflows need a bounded multi-step record, not just one command

That is the product shape.

The examples should feel like proofs of those six reasons, not like a file browser of unrelated demos.

For NVIDIA specifically, the same rule applies:

1. `Model`: ConfigHub can represent the layered GPU-oriented stack correctly
2. `Publish`: ConfigHub can preserve the bundle facts that come out of the recipe and delivery path
3. `Apply`: ConfigHub can deliver that stack through a real target
4. `Verify`: a real GPU-capable environment shows the result, not just structural YAML proof

Right now the incubator tells the first story better than the second and third.

For AICR bundles specifically, the mental model should be:

1. the layered recipe produces a deployable bundle for a target
2. ConfigHub preserves the bundle URI, digest, and publication context
3. ConfigHub preserves integrity and supply-chain evidence such as checksums, SBOM references, and attestation references
4. a downstream deployer consumes that same bundle digest
5. a later operator can inspect both the recipe provenance and the bundle evidence without guessing

Right now the incubator explains that shape well, but proves it only partly through sample evidence.

## Plan

### Phase 1: Add one explicit “Why ConfigHub?” incubator page

Create one short incubator page that answers:

- what ConfigHub adds beyond GitOps controllers
- what “import vs inspect vs mutate vs apply vs model” means
- which one example to open first for each reason

This page should be shorter and simpler than the current incubator README.

### Phase 2: Collapse the front-door choice into four reasons

Make the incubator landing docs route first by:

- `Import and inspect existing config`
- `Mutate operational config`
- `Apply real changes to a real app`
- `Model layered or governed config`

Only after that should the docs branch into Argo, Flux, Spring Boot, layered recipes, or procedure design.

As part of that rewrite, the front door should also introduce one delivery matrix:

- direct Kubernetes
- Flux OCI
- Argo OCI
- renderer-only companion paths

### Phase 3: Add a one-line “This proves X, not Y” contract to more READMEs

The best examples already imply this.

Make it explicit in more places:

- what this example proves
- what it does not prove
- why you would choose it over the neighboring example

### Phase 4: Add a tiny glossary where the front door can point

Do not expand every README with definitions.

Instead add one incubator-local glossary for terms that a new user will otherwise trip on:

- WET
- brownfield
- variant
- clone link
- renderer
- target
- worker
- bundle
- deployment variant

### Phase 5: Tighten package-level AI guides to mirror the new-user route

The AI guides should choose examples by reason first, not by package familiarity.

The strongest candidates for this next pass are:

- `incubator/global-app-layer/AI_START_HERE.md`
- `incubator/gitops-import-argo/AI_START_HERE.md`
- `incubator/gitops-import-flux/AI_START_HERE.md`

### Phase 5b: Make OCI delivery modes explicit

The repo should stop making readers infer this from scattered example docs.

Make one shared explanation of:

- why direct apply is still the simplest live proof
- why Flux OCI is the current standard controller-oriented bundle path
- why Argo OCI is the right next standard for Argo
- why `ArgoCDRenderer` should stay labeled as renderer-oriented until an OCI-backed Argo path exists

### Phase 6: Add “expected first value” timing to the standard examples

For the strongest front-door examples, say what the user should see first and roughly when:

- Argo: guestbook apps healthy in a few minutes
- Flux: `podinfo` healthy in a few minutes
- Spring Boot: real deploy/apply proof after cluster plus worker setup

This keeps the examples accountable to the 5-10 minute standard.

### Phase 7: Add an explicit NVIDIA real e2e track

The current plan is not strong enough here. It treats NVIDIA mostly as a layered-model story.

We need one explicit incubator plan for a real NVIDIA-shaped end-to-end proof:

- real GPU-capable cluster or cluster path
- real non-`Noop` target
- real ConfigHub apply
- real deployment result
- real verification that checks the deployed outcome rather than only the stored config

This does not need to start as the biggest possible GPU stack. It should start as the smallest believable real proof that still deserves the NVIDIA framing.

Candidate direction:

- keep `gpu-eks-h100-training` as the model and layering example
- add or evolve one separate `gpu-eks-h100-training-real-e2e` style path only when it can honestly prove real deployment and verification
- until then, keep the current GPU example labeled as structural or both-options, not as a fully proven real e2e standard

### Phase 8: Add an explicit AICR bundle real-proof track

The current plan is also not strong enough for bundles.

We need one explicit incubator plan for a believable AICR bundle proof:

- real bundle publication record with URI or OCI reference
- real bundle digest
- real publication context linking back to the deployment variant, target, and recipe manifest or unit revisions
- real integrity evidence such as checksum material and verification result
- real supply-chain evidence such as SBOM references, attestation references, provenance statement references, or signature result
- real downstream handoff that consumes the same published digest

This does not need to start with the biggest GPU stack either. It should start as the smallest honest bundle proof that still deserves the AICR framing.

Candidate direction:

- keep `bundle-evidence-sample` as the honest fixture-backed explainer
- keep `05-bundle-publication-walkthrough.md` as the staged product/story document
- add or evolve one separate real bundle proof only when there is a durable publication record and inspection path that is not just fixture output
- until then, keep the bundle story labeled as walkthrough plus sample, not as a fully proven real bundle flow

## Highest-Value Next Actions

1. Create one incubator-local `WHY_CONFIGHUB.md` or equivalent front-door explainer — **DONE** (see `incubator/WHY_CONFIGHUB.md`)
2. Rewrite the top of `incubator/README.md` around the four reasons above — **DONE**
3. Add a tiny glossary page and link to it from the landing docs — **DONE** (included in `WHY_CONFIGHUB.md`)
4. Tighten `global-app-layer/AI_START_HERE.md` so it routes by reason more explicitly — **DONE**
5. Write a dedicated NVIDIA real-e2e plan that names the cluster, target, apply path, and verification contract explicitly
6. Write a dedicated AICR bundle plan that names the publication record, digest source, integrity evidence, supply-chain evidence, and handoff contract explicitly — **PARTIALLY DONE** (bundle docs updated, Argo OCI spec written)
7. Standardize the delivery-mode language so the front door distinguishes direct apply, Flux OCI, Argo OCI, and renderer-only paths — **DONE**

## Working Rule For Future Example Docs

For every incubator example, a brand-new user should be able to answer these three questions in under 30 seconds:

1. Why does this example exist?
2. Why would I run this instead of the neighboring example?
3. Is this import, inspect, mutate, apply, model, or procedure?

For NVIDIA-shaped examples, add a fourth question:

4. Is this only structural, or is it a fully real end-to-end proof?

For NVIDIA bundle-shaped examples, add a fifth question:

5. Is this a real published bundle flow, or a walkthrough/sample explaining what the real flow should preserve?
