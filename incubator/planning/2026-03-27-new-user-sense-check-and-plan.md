# New-User Sense Check And Plan

**Date:** 2026-03-27  
**Repo:** `/Users/alexis/Public/github-repos/examples`

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

### Phase 6: Add “expected first value” timing to the standard examples

For the strongest front-door examples, say what the user should see first and roughly when:

- Argo: guestbook apps healthy in a few minutes
- Flux: `podinfo` healthy in a few minutes
- Spring Boot: real deploy/apply proof after cluster plus worker setup

This keeps the examples accountable to the 5-10 minute standard.

## Highest-Value Next Actions

1. Create one incubator-local `WHY_CONFIGHUB.md` or equivalent front-door explainer
2. Rewrite the top of `incubator/README.md` around the four reasons above
3. Add a tiny glossary page and link to it from the landing docs
4. Tighten `global-app-layer/AI_START_HERE.md` so it routes by reason more explicitly

## Working Rule For Future Example Docs

For every incubator example, a brand-new user should be able to answer these three questions in under 30 seconds:

1. Why does this example exist?
2. Why would I run this instead of the neighboring example?
3. Is this import, inspect, mutate, apply, model, or procedure?
