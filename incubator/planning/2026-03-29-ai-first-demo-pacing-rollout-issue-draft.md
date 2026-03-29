# Issue: Apply AI-first demo pacing standard to all examples

## Summary

We have 31 `AI_START_HERE.md` files across the examples repo. Most were written before we learned how to make AI demos pause properly. They need to be updated to follow the AI-first demo pacing standard.

This count was verified from the repo on 2026-03-29.

## The standard

The standard is:

1. **Explicit pause block at the top** — without this, Claude races through
2. **Stage-based structure** — each stage has title, command, explanation, GUI link, and PAUSE marker
3. **Three-part GUI annotations** — what it shows now, what is missing, and the feature ask

## Files to update

```text
examples/AI_START_HERE.md
examples/incubator/AI_START_HERE.md
examples/incubator/apptique-argo-app-of-apps/AI_START_HERE.md
examples/incubator/apptique-argo-applicationset/AI_START_HERE.md
examples/incubator/apptique-flux-monorepo/AI_START_HERE.md
examples/incubator/artifact-workflow/AI_START_HERE.md
examples/incubator/combined-git-live/AI_START_HERE.md
examples/incubator/connect-and-compare/AI_START_HERE.md
examples/incubator/connected-summary-storage/AI_START_HERE.md
examples/incubator/custom-ownership-detectors/AI_START_HERE.md
examples/incubator/demo-data-adt/AI_START_HERE.md
examples/incubator/fleet-import/AI_START_HERE.md
examples/incubator/flux-boutique/AI_START_HERE.md
examples/incubator/gitops-import-argo/AI_START_HERE.md
examples/incubator/gitops-import-flux/AI_START_HERE.md
examples/incubator/global-app-layer/AI_START_HERE.md
examples/incubator/global-app-layer/bundle-evidence-sample/AI_START_HERE.md
examples/incubator/global-app-layer/frontend-postgres/AI_START_HERE.md
examples/incubator/global-app-layer/gpu-eks-h100-training/AI_START_HERE.md
examples/incubator/global-app-layer/realistic-app/AI_START_HERE.md
examples/incubator/global-app-layer/single-component/AI_START_HERE.md
examples/incubator/graph-export/AI_START_HERE.md
examples/incubator/import-from-bundle/AI_START_HERE.md
examples/incubator/import-from-live/AI_START_HERE.md
examples/incubator/lifecycle-hazards/AI_START_HERE.md
examples/incubator/orphans/AI_START_HERE.md
examples/incubator/platform-example/AI_START_HERE.md
examples/incubator/platform-write-api/AI_START_HERE.md
examples/incubator/springboot-platform-app-centric/AI_START_HERE.md
examples/incubator/springboot-platform-app/AI_START_HERE.md
examples/incubator/watch-webhook/AI_START_HERE.md
```

## Acceptance criteria

- [ ] Every `AI_START_HERE.md` has the CRITICAL pause block at the top
- [ ] Every `AI_START_HERE.md` uses stage-based structure with PAUSE markers
- [ ] Every `AI_START_HERE.md` has a suggested prompt section
- [ ] GUI-relevant stages have three-part annotations: GUI now, GUI gap, GUI ask
- [ ] The standard is linked from a repo-local doc such as `incubator/AGENTS.md`, `incubator/AI-README-FIRST.md`, or a dedicated standard doc

## Why this matters

AI-first means the AI is the developer experience. The demo prompt is the product. If someone pastes the suggested prompt and the demo races through without pausing, they learn nothing and do not try ConfigHub.

Every example should be demoable by pasting one prompt.

## Suggested approach

1. Start with the highest-value front doors: `springboot-platform-app-centric`, `springboot-platform-app`, `gitops-import-argo`, `gitops-import-flux`, and `global-app-layer/single-component`
2. Update in batches of 3-5 files, grouped by family
3. Test each update by running the suggested prompt and checking that the AI actually pauses
4. File GUI feature issues discovered during testing

## Labels

`examples`, `ai-first`, `documentation`
