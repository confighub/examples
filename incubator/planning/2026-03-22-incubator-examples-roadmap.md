# Roadmap: Incubator Examples After cub-scout Adaptation

**Date:** 2026-03-22
**Status:** Active
**Scope:** `/Users/alexis/Public/github-repos/examples/incubator`

## Summary

The incubator set now has a strong front half and a much broader middle.

The no-cluster path is strong enough to stand on its own:

- `connect-and-compare`
- `import-from-bundle`
- `fleet-import`
- `demo-data-adt`
- `lifecycle-hazards`
- `connected-summary-storage`
- `artifact-workflow`

The live import and live evidence path is also in place:

- `import-from-live`
- `combined-git-live`
- `gitops-import-argo`
- `gitops-import-flux`
- `custom-ownership-detectors`
- `graph-export`
- `orphans`
- `watch-webhook`
- `flux-boutique`

The app-style set is present, but still needs trustworthy live validation:

- `apptique-flux-monorepo`
- `apptique-argo-applicationset`
- `apptique-argo-app-of-apps`

The layered package remains the advanced second stop:

- `global-app-layer`

Top-level entry docs now point first to the no-cluster evidence spine, then to the live import and live evidence examples, then to worker examples and the deeper layered material.

## What Is Done

The following work is already merged on `main`.

### Offline and evidence-first incubator examples

- `incubator/connect-and-compare`
- `incubator/import-from-bundle`
- `incubator/fleet-import`
- `incubator/demo-data-adt`
- `incubator/lifecycle-hazards`
- `incubator/connected-summary-storage`
- `incubator/artifact-workflow`

### Live import, ownership, topology, and integration examples

- `incubator/import-from-live`
- `incubator/combined-git-live`
- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`
- `incubator/custom-ownership-detectors`
- `incubator/graph-export`
- `incubator/orphans`
- `incubator/watch-webhook`
- `incubator/flux-boutique`

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

### 1. Live validation of the app-style examples

This is still the main unfinished example-quality task.

The app-style examples are documented and structurally validated, but they still need a clean end-to-end live pass on a healthy Docker and kind runtime before promotion is considered.

Priority targets:

- `incubator/apptique-flux-monorepo`
- `incubator/apptique-argo-applicationset`
- `incubator/apptique-argo-app-of-apps`

### 2. Promotion criteria and promotion decisions

The incubator set is broad now. The next question is not only what else to add, but also what should graduate, what should stay incubator, and what should remain companion material only.

### 3. Dedicated kubeconfig follow-through for older live examples

A safer dedicated-kubeconfig pattern is now proven in the newer live examples and in the safety pass. The rest of the live example surface should converge on that pattern over time.

### 4. Upstream cub-scout contract cleanup

The following upstream mismatches were found and filed:

- `confighub/cub-scout#331` for `scan --file --json` exit semantics
- `confighub/cub-scout#332` for `drift` docs and command contract mismatch
- `confighub/cub-scout#333` for custom ownership detectors applying in `map list` but not consistently in `explain` or `trace`
- `confighub/cub-scout#334` for `trace` misclassifying a native Deployment as Flux in the `orphans` fixture
- `confighub/cub-scout#335` for `map orphans` not surfacing the fixture CronJob
- `confighub/cub-scout#336` for the `workflows/fleet-demo` README overclaiming differences in the current prebuilt bundles
- `confighubai/confighub#4025` for dedicated kubeconfig use in live examples and incubator workflows

Until the drift issue is resolved, do not promote a `drift` example into `examples`.

## Recommended Next Sequence

### Phase 1: Re-verify the no-cluster and smaller live examples

Do a clean testing pass over the no-cluster spine and the smaller live examples with the current dedicated-kubeconfig pattern.

Success means:

- each example still runs as documented
- `verify.sh` matches the real current output
- the README claims stay aligned with the runtime behavior
- no example depends on ambient kube state

### Phase 2: Finish trustworthy live validation of the app-style examples

Run one clean live validation pass for the app-style examples on a healthy runtime.

Success means:

- each example can be applied to a suitable cluster
- each example has a working `verify.sh` path
- the evidence in the README matches what the runtime actually produced
- no example depends on overclaiming controller status or ownership

### Phase 3: Tighten docs from real validation output

If live validation exposes mismatches, fix the examples first, then update the docs.

If live validation passes, add only the minimum doc updates needed to reflect exact observed behavior.

### Phase 4: Decide promotions

After live validation, decide which examples should stay incubator and which should move toward stable examples.

The strongest promotion candidates are likely to be:

- one no-cluster example
- one live import or live evidence example
- possibly one app-style example

### Phase 5: Continue selective adaptation from cub-scout

Only continue pulling examples from `cub-scout` if they add a new capability or operator story that the current incubator set still lacks.

Good candidates should be:

- real
- small enough to maintain
- consistent with the evidence-first standard
- not already covered by a stronger incubator example

## Promotion Heuristics

A candidate is ready to consider for promotion when it meets all of these:

- runnable end to end
- clear read-only-first entry path
- explicit mutation boundaries
- explicit evidence checklist
- stable enough to survive a fresh local run
- no important gap between docs and actual command contract
- safe cluster handling if it is a live example

## Current Map

### Best no-cluster examples

- `incubator/connect-and-compare`
- `incubator/import-from-bundle`
- `incubator/fleet-import`
- `incubator/demo-data-adt`
- `incubator/lifecycle-hazards`
- `incubator/connected-summary-storage`
- `incubator/artifact-workflow`

### Best live import and evidence examples

- `incubator/import-from-live`
- `incubator/combined-git-live`
- `incubator/gitops-import-argo`
- `incubator/gitops-import-flux`
- `incubator/custom-ownership-detectors`
- `incubator/graph-export`
- `incubator/orphans`
- `incubator/watch-webhook`
- `incubator/flux-boutique`

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

This roadmap supersedes the earlier assumption that the next major work item was primarily more adaptation. The incubator set is now broad enough that validation, safety, and promotion decisions matter as much as adding new examples.
