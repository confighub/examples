# AI Handover: Mainstream Runnable Examples from cub-scout Material

## What This Work Is About

The goal is to make official, runnable examples in `confighub/examples` carry the current GitHub + Argo/Flux + AI/CLI + ConfigHub wedge.

Do not center the work on `global-app-layer`. That package remains important, but as the advanced second stop.

Do not treat `cub-scout` as the main surface. Treat it as a source of strong example material to adapt into `confighub/examples`.

## Main Decision

The official GitOps Import docs remain the front door.

The concrete runnable examples behind that story should live in `confighub/examples`, not primarily in `cub-scout`.

The first two examples to prioritize are:

- Argo import, using `examples/gitops-import` as the home
- Flux import, adapted into a sibling example in `examples`

For LIVE verification, use this model:

- `kubectl` for raw cluster facts
- ConfigHub for import and renderer facts
- `cub-scout` for live ownership and GitOps context facts

## Current State

The `examples` repo already has a real Argo anchor:

- `examples/gitops-import/README.md`
- `examples/gitops-import/bin/create-cluster`
- `examples/gitops-import/bin/install-argocd`
- `examples/gitops-import/bin/install-worker`
- `examples/gitops-import/bin/setup-apps`

The `examples` repo entry points currently still send users to `cub-scout` for the concrete Argo and Flux paths:

- `examples/README.md`
- `examples/AI_START_HERE.md`
- `examples/incubator/README.md`
- `examples/incubator/global-app-layer/README.md`

The best current source material in `cub-scout` is:

- `cub-scout/examples/argo-import-confighub-demo/README.md`
- `cub-scout/examples/flux-import-confighub-demo/README.md`
- `cub-scout/examples/apptique-examples/README.md`

## What Has Already Been Tightened Elsewhere

The `examples` repo has already been adjusted so that:

- `global-app-layer` is no longer treated as the front door for the current wedge
- the official GitOps Import docs are linked as the best first stop
- `ArgoCDRenderer` is described honestly as a render or hydration path rather than as real Argo-managed workload sync

There is also a related issue in `confighub`:

- `confighubai/confighub#4007`

That issue is about `cub` help and test-guide wording for AI-first GitOps import workflows. It is not the migration plan for the `examples` repo.

There is also a useful verification readout from the Argo import path:

- `ArgoCDRenderer` can provide a partial proof when the unit itself is an ArgoCD `Application`
- that can prove renderer acceptance and Argo-side refresh or reconcile timing
- it does not by itself prove that the specific ConfigHub action created new workloads

Use that distinction in the docs. Do not overclaim workload creation from renderer evidence.

## What To Do Next

The next practical slice is:

1. Rewrite `examples/gitops-import/README.md` so it becomes the canonical runnable Argo import example.
2. Consolidate the duplicate nested README in `examples/gitops-import/gitops-import/README.md`.
3. Create the Flux sibling example in `examples`, adapting the `cub-scout` Flux import demo into the same shape.
4. Update the top-level `examples` repo docs so the runnable Argo and Flux examples in `examples` become the default concrete path behind the official GitOps Import docs.

## Documentation Standard to Apply

For each promoted example, make sure the docs say:

- what the example is for
- what it reads
- what it writes
- what mutates ConfigHub
- what mutates live infrastructure
- what success looks like
- what evidence to inspect next

Start with read-only checks. Prefer direct evidence over narrative. Do not overclaim live reconciliation from import or render evidence alone.

For LIVE examples, also make sure the docs separate:

- cluster facts
- ConfigHub facts
- `cub-scout` facts

## Important Boundaries

Do not migrate all of `cub-scout`.

Do not make `cub-scout` the official front door.

Do not pull `global-app-layer` back into the role of mainstream import example.

Do not center the story on ConfigHub apply or on ConfigHub status as runtime truth.

## Recommended First Files

Start with these:

- `examples/gitops-import/README.md`
- `examples/gitops-import/gitops-import/README.md`
- `examples/README.md`
- `examples/START_HERE.md`
- `examples/AI_START_HERE.md`

Then move to the Flux sibling once the Argo path in `examples` is clean.

## Why This Matters

Right now the strongest concrete examples for the wedge live in `cub-scout`, while the official examples repo still reads as the primary home for ConfigHub examples overall. This plan closes that gap by making the official runnable path live where users already expect it to live.
