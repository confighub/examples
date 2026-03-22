# Roadmap: Incubator Examples After cub-scout Adaptation

**Date:** 2026-03-22
**Status:** Active
**Scope:** `/Users/alexis/Public/github-repos/examples/incubator`

## Summary

The incubator set now has a coherent front half.

The no-cluster path is strong enough to stand on its own:

- `connect-and-compare`
- `import-from-bundle`
- `fleet-import`
- `demo-data-adt`

The live GitOps import path is also in place:

- `gitops-import-argo`
- `gitops-import-flux`

The app-style set is present, but still needs trustworthy live validation:

- `apptique-flux-monorepo`
- `apptique-argo-applicationset`
- `apptique-argo-app-of-apps`

The layered package remains the advanced second stop:

- `global-app-layer`

Top-level entry docs now point first to the no-cluster evidence spine, then to the live import examples, then to worker examples and the deeper layered material.

## What Is Done

The following work is already merged on `main`.

### Offline and evidence-first incubator examples

- `incubator/connect-and-compare`
- `incubator/import-from-bundle`
- `incubator/fleet-import`
- `incubator/demo-data-adt`
- `incubator/combined-git-live`

### Live import examples

- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`

### App-style examples adapted from cub-scout

- `incubator/apptique-flux-monorepo`
- `incubator/apptique-argo-applicationset`
- `incubator/apptique-argo-app-of-apps`

### Layered examples and deployment-variant work

- `incubator/global-app-layer/gpu-eks-h100-training` now models direct and Flux deployment variants explicitly

### Supporting planning and integration docs

- `incubator/cub-proc`
- `incubator/vmcluster-from-scratch.md`
- `incubator/vmcluster-nginx-path.md`

### Repo entry points

The following now reflect the no-cluster-first route before live import:

- `README.md`
- `AI_START_HERE.md`
- `START_HERE.md`
- `incubator/README.md`
- `incubator/AI_START_HERE.md`

## What Is Not Done

### 1. Live validation of app-style examples

This is the main unfinished example-quality task.

The app-style examples are documented and structurally validated, but the latest live validation attempt was blocked by the local Docker or kind runtime becoming sticky. The examples should be exercised again on a healthy local runtime before promotion is considered.

Priority targets:

- `incubator/apptique-flux-monorepo`
- `incubator/apptique-argo-applicationset`
- `incubator/apptique-argo-app-of-apps`

### 2. Promotion criteria and promotion decisions

The incubator set is much broader now. The next question is not only "what else can we add" but also "what should graduate, what should stay incubator, and what should remain companion material only."

### 3. Upstream cub-scout contract cleanup

Two upstream mismatches were found and filed:

- `confighub/cub-scout#331` for `scan --file --json` exit semantics
- `confighub/cub-scout#332` for `drift` docs and command contract mismatch

Until those are resolved, do not promote a `drift` example into `examples`.

## Recommended Next Sequence

### Phase 1: Finish trustworthy live validation

Run one clean live validation pass for the app-style examples on a healthy runtime.

Success means:

- each example can be applied to a suitable cluster
- each example has a working `verify.sh` path
- the evidence in the README matches what the runtime actually produced
- no example depends on overclaiming controller status or ownership

### Phase 2: Tighten docs from real validation output

If live validation exposes mismatches, fix the examples first, then update the docs.

If live validation passes, add only the minimum doc updates needed to reflect exact observed behavior.

### Phase 3: Decide promotions

After live validation, decide which examples should stay incubator and which should move toward stable examples.

The strongest promotion candidates are likely to be:

- one no-cluster example
- one live import example
- possibly one app-style example

### Phase 4: Continue selective adaptation from cub-scout

Only continue pulling examples from `cub-scout` if they add a new capability or operator story that the current incubator set still lacks.

Good candidates should be:

- real
- small enough to maintain
- consistent with the evidence-first standard

## Promotion Heuristics

A candidate is ready to consider for promotion when it meets all of these:

- runnable end to end
- clear read-only-first entry path
- explicit mutation boundaries
- explicit evidence checklist
- stable enough to survive a fresh local run
- no important gap between docs and actual command contract

## Current Map

### Best no-cluster examples

- `incubator/connect-and-compare`
- `incubator/import-from-bundle`
- `incubator/fleet-import`
- `incubator/demo-data-adt`

### Best live import examples

- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`

### Best app-style examples

- `incubator/apptique-flux-monorepo`
- `incubator/apptique-argo-applicationset`
- `incubator/apptique-argo-app-of-apps`

### Advanced follow-on material

- `incubator/global-app-layer`
- `incubator/cub-proc`
- `incubator/vmcluster-from-scratch.md`
- `incubator/vmcluster-nginx-path.md`

## Notes

This roadmap supersedes the earlier planning assumption that the next major work item was moving the stable `gitops-import` example itself. The current incubator set has grown enough that the next highest-value work is validation, tightening, and promotion decisions, not just more adaptation.
