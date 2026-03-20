# Plan: Bring Selected cub-scout Examples into Mainstream ConfigHub Runnable Examples

**Date:** 2026-03-20
**Status:** Proposed
**Scope:** `confighub/examples` as the official runnable examples surface, with selected material adapted from `cub-scout`
**Origin:** Follow-on from the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge discussion

## Summary

The focus should be on official, runnable examples in `confighub/examples`.

`cub-scout` should be treated as a source of strong example material, not as the main user-facing surface for the current wedge. The official GitOps Import docs should stay the front door, but the runnable examples behind that story should live in `confighub/examples`.

The immediate goal is to make Argo import and Flux import feel like first-class ConfigHub examples, with the same operator model, the same evidence model, and the same AI-first affordances that worked well in the `global-app-layer` package.

## Problem

Today the current wedge is split across three places:

- the published GitOps Import docs
- the `examples` repo entry points
- the concrete Argo, Flux, Helm, and app-style examples in `cub-scout`

This works, but it does not yet feel like one coherent product surface. A user can start in the official docs and then get sent to `cub-scout` for the concrete walkthroughs, while the `examples` repo still reads as the primary home for ConfigHub examples overall.

That split creates three problems.

First, the official runnable path is not obvious. The strongest concrete Argo and Flux examples exist, but not in the repo that users naturally read as the official examples catalog.

Second, the examples do not yet have one standard shape. The `global-app-layer` package has gained clear AI-first guidance, explicit mutation boundaries, and exact verification checkpoints. The `cub-scout` examples are strong, but they still read more like companion demos than like the canonical examples behind the docs.

Third, the current wedge risks feeling broader and more fragmented than it needs to be. The immediate story is not "all ConfigHub capabilities." The immediate story is GitHub, Argo or Flux, AI and CLI as the operator path, and ConfigHub as the place that ingests WET config, organizes it, validates it, and shows evidence.

## Decision

The official runnable path for the current wedge should live in `confighub/examples`.

Selected `cub-scout` examples should be adapted into `confighub/examples` and made to conform to the `examples` repo standard for positioning, verification, and tool-assisted use.

`cub-scout` should remain valuable, but as an upstream source of example material and broader explorations rather than as the primary destination for the mainstream path.

## Guiding Principles

The first principle is that the docs page remains the front door. The published GitOps Import docs are the right public anchor. The work here is about making the official runnable examples behind that story live in the `examples` repo.

The second principle is that import and evidence come first. The mainstream examples should emphasize import, inspection, validation, scan, comparison, and evidence. They should not depend on apply or status claims to justify their value.

The third principle is consistency. Argo import and Flux import should feel like the same example family. The difference should be the source objects and controller behavior, not the operator model or the proof model.

The fourth principle is that AI-first support means safe and explicit workflows. Every promoted example should start with read-only checks, should state what mutates ConfigHub and what mutates live infrastructure, and should say exactly what evidence to inspect next.

The fifth principle is that LIVE examples should use a layered verification model. Direct cluster commands should prove raw runtime facts. ConfigHub should prove import and rendered WET configuration facts. `cub-scout` should be used where helpful to prove live ownership and GitOps context facts.

The sixth principle is that `global-app-layer` remains the advanced second stop. It should not return to being the front door for the current wedge.

## Current Anchors

The existing `examples` repo already has the right starting point for the Argo path:

- `examples/gitops-import/README.md`
- `examples/gitops-import/bin/create-cluster`
- `examples/gitops-import/bin/install-argocd`
- `examples/gitops-import/bin/install-worker`
- `examples/gitops-import/bin/setup-apps`

The existing `examples` repo also already points users toward `cub-scout` for concrete Argo and Flux import demos:

- `examples/README.md`
- `examples/AI_START_HERE.md`
- `examples/incubator/README.md`
- `examples/incubator/global-app-layer/README.md`
- `examples/incubator/global-app-layer/confighub-aicr-value-add.md`

The current `cub-scout` source material is strongest in:

- `cub-scout/examples/argo-import-confighub-demo/README.md`
- `cub-scout/examples/flux-import-confighub-demo/README.md`
- `cub-scout/examples/apptique-examples/README.md`

## Target Outcome

The `examples` repo should become the place where a user can follow the mainstream wedge end to end:

- start at the official GitOps Import docs
- land in a runnable Argo import example in `examples`
- land in a runnable Flux import example in `examples`
- follow one Helm-oriented path in `examples`
- follow one app-style path in `examples`

At that point, `cub-scout` is still relevant, but mainly as the upstream place where some of the example material originated and where broader explorations continue.

## Migration Plan

### Phase 1: Promote `examples/gitops-import` into the canonical Argo import example

`examples/gitops-import` is already the right home. The work here is editorial, structural, and verification-oriented.

The top-level README in `examples/gitops-import` should be rewritten so it clearly explains:

- what the example is for
- what it reads
- what it writes
- what mutates ConfigHub
- what mutates live infrastructure
- what success looks like
- what evidence to inspect after discover and import

It should begin with read-only checks, then move into cluster setup, then GitOps discovery, then GitOps import, then verification.

It should also make the current technical boundary explicit: imported WET config and renderer evidence are valuable, but they are not the same thing as proving live controller reconciliation.

For LIVE verification, it should use the three-part model:

- direct cluster evidence
- ConfigHub evidence
- `cub-scout` ownership and GitOps context evidence

The duplicate nested README at `examples/gitops-import/gitops-import/README.md` should be consolidated so there is one authoritative path.

### Phase 2: Create a first-class Flux import sibling in `examples`

The Flux path should become an `examples` repo example rather than a link out to `cub-scout`.

The source material should come from `cub-scout/examples/flux-import-confighub-demo`, but the result should be adapted to match the shape of the Argo example in `examples`.

That means:

- same doc structure
- same mutation model
- same read-only-first path
- same verification style
- same distinction between imported evidence and runtime truth

The likely destination is a new stable example directory next to `examples/gitops-import`.

### Phase 3: Update the top-level `examples` repo entry points

Once the Argo and Flux paths are both in `examples`, the top-level docs should stop treating `cub-scout` as the default concrete destination for the current wedge.

The main files to update are:

- `examples/README.md`
- `examples/START_HERE.md`
- `examples/AI_START_HERE.md`
- `examples/incubator/README.md`
- `examples/incubator/global-app-layer/README.md`

These should point in this order:

- official GitOps Import docs
- runnable Argo import example in `examples`
- runnable Flux import example in `examples`
- Helm-oriented path in `examples`
- app-style path in `examples`

`cub-scout` can still be referenced, but no longer as the primary concrete path for the wedge.

### Phase 4: Align the Helm path with the same example standard

The likely anchor remains `examples/helm-platform-components`.

The goal is not to force Helm into the import story. The goal is to make the operator experience and documentation standard feel consistent:

- clear purpose
- read-only first checks
- mutation boundaries
- verification checkpoints
- evidence model

### Phase 5: Bring one app-style pattern into `examples`

The best source material is `cub-scout/examples/apptique-examples`, but only one pattern should be promoted first.

The first promoted app-style example should be chosen for clarity and fit with the wedge, not for breadth. One strong Flux monorepo or one strong Argo ApplicationSet path is enough for the first slice.

The promoted example should reinforce the same product story:

- existing GitOps layout
- ConfigHub import and organization
- validation and evidence
- comparison with live state

### Phase 6: Keep `global-app-layer` as advanced follow-on material

The `global-app-layer` package should continue to carry:

- layered recipes
- deployment units
- downstream specialization
- NVIDIA-shaped configuration chains

It should not carry the mainstream import wedge as its primary role.

## Example Standard for Promoted Runnable Examples

Every promoted example in `examples` should meet the same standard.

It should be runnable end to end. It should start with read-only checks. It should explicitly say what mutates ConfigHub and what mutates live infrastructure. It should tell the operator what to inspect next. It should define success using evidence rather than narration. It should be safe to drive with an AI assistant using the CLI as the main interface.

The point is not to create the same file bundle everywhere. The point is to make the behavior and documentation quality consistent.

For LIVE examples, the verification model should also be consistent. `kubectl` should prove raw cluster truth. ConfigHub should prove import and intended-state facts. `cub-scout` should prove live ownership and GitOps context when that interpretation matters.

## Non-Goals

This plan does not attempt to move all of `cub-scout` into `examples`.

This plan does not attempt to make the import path depend on ConfigHub apply or on ConfigHub being the runtime source of truth.

This plan does not attempt to replace the official GitOps Import docs. Those remain the front door.

This plan does not attempt to turn `global-app-layer` into the mainstream path.

## Risks and Constraints

The main risk is duplicating examples without improving clarity. The migration should not become a shallow copy of `cub-scout` into `examples`. The examples need to be adapted so the official path feels simpler and more coherent than before.

The second risk is overclaiming runtime truth. The examples should continue to show direct cluster evidence where runtime behavior matters, rather than relying on import status alone.

The third risk is treating `cub-scout` as a replacement for direct cluster commands. It should be used as an additional interpretation layer for ownership and GitOps context, not as the only live evidence surface.

The fourth risk is creating too many parallel first paths. Argo import and Flux import are the first two priorities. Helm and app-style paths should follow, not compete for front-door status immediately.

## First Implementation Slice

The smallest high-value slice is:

1. rewrite `examples/gitops-import/README.md` into the canonical Argo import example
2. remove or consolidate `examples/gitops-import/gitops-import/README.md`
3. create the Flux sibling in `examples`, adapted from the existing `cub-scout` Flux import demo
4. update the top-level `examples` entry docs so Argo import and Flux import in `examples` become the default runnable paths behind the official GitOps Import docs

## Success Criteria

The plan is successful when a user can start from the official GitOps Import docs and then follow an Argo import or Flux import example entirely within `confighub/examples` without feeling that they have moved into a companion or experimental repo.

It is also successful when those examples answer "why import?" quickly, are safe to drive from a tool-assisted session, and prove value with imported WET config, validation or scan results, comparison, and evidence rather than with fragile apply or status claims.

## Related Context

Related issue in `confighub`:

- `confighubai/confighub#4007` — help and documentation change for AI-first GitOps import workflows

This issue is adjacent to the current plan, but it is not the plan itself. That issue improves `cub` help text and the ArgoCD import test guide. This plan is about making the runnable example surface in `confighub/examples` match the current wedge.
