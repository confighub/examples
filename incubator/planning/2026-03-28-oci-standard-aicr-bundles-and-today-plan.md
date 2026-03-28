# OCI Standard, AICR Bundles, and Today Plan

**Date:** 2026-03-28  
**Repo:** `/Users/alexis/Public/github-repos/examples`  
**Scope:** `/Users/alexis/Public/github-repos/examples/incubator`

## Why This Plan Exists

The recent chat clarified three important points:

1. real end to end means real apply and real verification, not `Noop`
2. ConfigHub should be treated as a real way to get config apps or recipes to targets through deployment
3. the controller-oriented bundle path should be OCI-shaped, not left as a vague future idea

The current repo already has part of this story:

- direct Kubernetes delivery is real and increasingly well proven
- `FluxOCI` is the clearest current controller-oriented delivery path
- `ArgoCDRenderer` is real, but it is a renderer-oriented companion path, not the standard Argo bundle story
- the NVIDIA AICR bundle material is useful, but still mostly walkthrough plus sample rather than a real proof

This plan makes the next direction explicit.

## Decisions Locked By The Chat

### 1. OCI is the standard controller bundle direction

For controller-oriented delivery in incubator examples, the standard path should be:

- ConfigHub materializes the deployment intent
- ConfigHub publishes or records a target-specific OCI bundle
- the controller consumes that OCI artifact
- the example verifies controller state and live workload state

### 2. Direct Kubernetes stays the standard non-bundle proof

Direct worker apply is still the simplest fully proven live-delivery path.

It should remain the standard answer when the goal is:

- smallest real apply proof
- fastest honest e2e
- no controller required

### 3. `FluxOCI` is the current standard OCI controller path

This is already the closest thing to a working standard in the repo.

Use it as the current controller-oriented reference path while tightening:

- bundle terminology
- verification contract
- example selection

### 4. `ArgoCDRenderer` is not the standard Argo OCI path

Keep saying this plainly:

- `ArgoCDRenderer` is renderer-oriented
- it expects Argo `Application` payloads
- it is not the same as publishing an OCI bundle and having Argo reconcile workloads from it

Do not let the docs drift back into implying otherwise.

### 5. Argo should move toward an OCI-backed standard path

The target state for Argo should be:

- ConfigHub publishes or records an OCI artifact
- ConfigHub writes or updates an Argo `Application` that points at that OCI source
- Argo performs the workload sync
- the example verifies Argo sync state and live cluster state against the same bundle digest

### 6. NVIDIA AICR should use the same bundle contract

The AICR bundle story should not be special-cased into hand-wavy language.

It should use the same clean contract:

- recipe provenance
- deployment variant
- target
- bundle URI and digest
- integrity and supply-chain evidence
- downstream controller handoff
- live verification where the example claims real e2e

## What “Standard OCI Path” Means

The standard OCI path for examples should answer these questions clearly:

1. What recipe or config app produced the deployable output?
2. Which target or delivery mode was selected?
3. What OCI reference and digest identify the published output?
4. Which controller objects consumed that exact reference or digest?
5. What live state proves the controller actually reconciled it?

If an example cannot answer those questions yet, it should not be described as a standard OCI proof.

## Workstreams

### Workstream 1: Make OCI a Standard Delivery Matrix in the Docs

Add one consistent delivery matrix across the incubator and layered docs:

- `direct-kubernetes`
- `flux-oci`
- `argo-oci` when real
- `argocd-render` as a non-standard companion path

Files that should converge on this language:

- `incubator/README.md`
- `incubator/AI_START_HERE.md`
- `incubator/global-app-layer/README.md`
- `incubator/global-app-layer/AI_START_HERE.md`
- `incubator/global-app-layer/how-it-works.md`
- `incubator/global-app-layer/contracts.md`

Success means:

- a new reader can tell which paths are standard and which are not
- `ArgoCDRenderer` is no longer confused with Argo OCI delivery
- the word `bundle` is attached to target-specific output, not used vaguely

### Workstream 2: Make One Small OCI Example Standard

Pick one smallest honest example to become the standard OCI wedge before the GPU story carries that weight.

Candidate order:

1. `incubator/global-app-layer/single-component`
2. `incubator/global-app-layer/frontend-postgres`
3. keep `gpu-eks-h100-training` as the domain-shaped follow-on, not the first standard proof

This standard small example should prove:

- target binding
- OCI-oriented controller path
- controller-side evidence
- live workload evidence
- bundle reference or digest visibility

If the smallest example cannot support this cleanly today, write the exact file-level spec before adding more complexity.

### Workstream 3: Tighten Flux OCI as the Current Standard

For Flux, the near-term goal is not a vague “bundle story.”

It is a concrete standard proof that includes:

- the right deployment variant for `FluxOCI` or `FluxOCIWriter`
- controller objects visible after apply
- live workloads visible after reconcile
- bundle reference or digest visible in the example output or inspection path
- docs that say Flux OCI is the standard controller path today

The strongest current domain-shaped example is:

- `incubator/global-app-layer/gpu-eks-h100-training`

But the small-example standard should be tightened first if possible.

### Workstream 4: Specify Argo OCI Cleanly Before Claiming It

Argo needs a new clean standard, not more wording around the existing renderer path.

The plan should define:

- target/provider shape for `ArgoOCI` or equivalent
- expected unit payload shape
- OCI publication or reference contract
- Argo `Application` source shape
- verification contract:
  - ConfigHub intent
  - bundle ref or digest
  - Argo sync and health evidence
  - cluster-side workload evidence

Until that exists, the docs must keep saying:

- `ArgoCDRenderer` is real
- `ArgoCDRenderer` is not the standard OCI bundle proof

### Workstream 5: Update NVIDIA AICR To Use The OCI Contract Cleanly

The AICR work should converge on:

- `gpu-eks-h100-training` as the domain-shaped layered recipe example
- direct delivery as the smallest proven live path
- Flux OCI as the first controller-oriented bundle path
- Argo OCI as the follow-on controller-oriented bundle path once real

The AICR bundle docs should then be tightened around the same standard questions:

1. what recipe produced the bundle
2. what target published it
3. what OCI ref and digest identify it
4. what evidence proves its integrity and supply-chain context
5. what controller consumed it
6. what live evidence proves deployment

### Workstream 6: Tidy Loose Ends

Loose ends that should be cleaned up while doing the above:

- replace vague `bundle hint` wording with more precise `bundle ref`, `bundle URI`, or `bundle digest` language where possible
- make `preflight-live.sh` and related docs expose delivery mode clearly
- keep `ArgoCDRenderer` examples labeled as renderer/hydration paths
- keep `bundle-evidence-sample` labeled as sample evidence until a real publication flow exists
- update handover and new-user plans so the repo points to the OCI direction consistently

## Definition Of Done

This strategy is ready when all of these are true:

1. one small example is the standard OCI proof
2. Flux OCI is clearly the standard controller path in current incubator docs
3. Argo OCI is described as the target-state standard and separated from `ArgoCDRenderer`
4. the NVIDIA AICR docs use the same bundle contract as the rest of the repo
5. no landing doc leaves a new reader guessing whether a path is direct apply, Flux OCI, Argo OCI, or renderer-only

## Today: Focused Sequential Work For Claude

Claude should work in this order and stop after each numbered step with a short evidence note.

### 1. Front Door First

Execute Phase 1 and Phase 2 from `2026-03-27-new-user-sense-check-and-plan.md`:

- create `incubator/WHY_CONFIGHUB.md`
- add a small incubator glossary
- rewrite the top of `incubator/README.md`
- rewrite the top of `incubator/AI_START_HERE.md`

Important requirement:

- route by reason first: import, mutate, apply, model
- include the delivery matrix language so the front door already distinguishes direct apply, Flux OCI, Argo OCI, and renderer-only paths

### 2. Standardize The OCI Language In Layered Docs

Then update the package-level layered docs so they all use the same matrix:

- `incubator/global-app-layer/README.md`
- `incubator/global-app-layer/AI_START_HERE.md`
- `incubator/global-app-layer/how-it-works.md`
- `incubator/global-app-layer/contracts.md`

The key output from this step is not implementation yet.
It is one stable shared vocabulary.

### 3. Pick And Tighten The Small OCI Standard

Pick the smallest honest OCI-capable example.

Prefer:

1. `incubator/global-app-layer/single-component`
2. then `frontend-postgres`
3. use `gpu-eks-h100-training` only as the domain-shaped follow-on

For the chosen example, either:

- tighten the docs and verification around the current Flux OCI path, or
- if the path is not there yet, write the exact implementation contract and stop

Do not jump to GPU first if the small proof is still fuzzy.

### 4. Update The AICR Bundle Contract

Then tighten the AICR bundle docs so they explicitly say:

- Flux OCI is the current controller-oriented bundle path
- Argo OCI is the target-state path
- `ArgoCDRenderer` is not the same thing
- `bundle-evidence-sample` is still sample evidence

Relevant files:

- `incubator/global-app-layer/04-bundles-attestation-and-todo.md`
- `incubator/global-app-layer/05-bundle-publication-walkthrough.md`
- `incubator/global-app-layer/06-bundle-evidence-gui-spec.md`
- `incubator/global-app-layer/bundle-evidence-sample/README.md`
- `incubator/global-app-layer/gpu-eks-h100-training/README.md`

### 5. Write The Argo OCI Spec

Do not over-implement if the provider path is not ready.

Write a crisp spec that names:

- target/provider name
- unit payload shape
- OCI publication record
- Argo `Application` source contract
- verification contract
- what counts as real proof

This is the handoff packet for the next implementation pass.

### 6. Cleanup And Re-Point Existing Plans

Before stopping:

- update the handover and new-user plans to point at the OCI strategy
- remove stale wording that implies renderer equals OCI delivery
- leave the repo with one obvious next step, not three competing ones

## Stop Conditions

If any step reveals that the underlying worker or provider path is not ready:

- keep the docs honest
- write the exact missing contract
- stop short of claiming a real proof

The goal for today is clarity and sequence, not pretending Argo OCI already exists.
