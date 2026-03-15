# Review of the `global-app-layer` Package Against the Recipes Spec

## Summary

The `global-app-layer` package is now a good staged implementation of the proposed recipes and layers convention.

It contains four examples:

- `single-component/`: proves the recipe model with one materialized chain
- `frontend-postgres/`: proves the same model at small app scope, with two components sharing the same layer spaces
- `realistic-app/`: proves the same model at fuller app scope, with backend, frontend, and postgres coordinated through one shared layer model
- `gpu-eks-h100-training/`: proves that the same recipe convention can express platform, accelerator, OS, and intent as explicit layers for a domain-shaped example

This is the right shape for onboarding because it gives us a small proof, a small app proof, a more recognisable app proof, and one domain-shaped proof side by side.

## What the Package Already Gets Right

### 1. Real materialized chains

Both examples use real units and real upstream relationships rather than describing a purely conceptual overlay model.

### 2. Explicit provenance

Both examples write explicit `Recipe` manifest units and keep placeholder-based base recipe files. That is exactly the right teaching pattern.

### 3. Ordered precedence is understandable

The chains are readable and reviewable. The examples teach a stable precedence model rather than a wide fan-out of hidden overlays.

### 4. Upgrade propagation is demonstrated

Both examples show that upstream changes can move down the chain without losing layer-specific mutations.

### 5. Verification exists

The examples are not just narrative. They include `verify.sh` scripts that check chain structure, mutations, and recipe manifest content.

## What Each Example Proves

### `single-component/`

This proves:

- one component can be expressed as a layered recipe chain
- a recipe manifest can remain explicit without requiring a new hard backend type
- upgrade propagation can be reviewed clearly

### `frontend-postgres/`

This proves:

- layer names can keep a consistent meaning across the app
- component-specific mutations can still differ within shared layer spaces
- an app-level recipe manifest can describe more than one component cleanly

This is an important step because it moves the model from "single unit mechanism" to "small app recipe".

### `realistic-app/`

This proves:

- the same layer model can coordinate backend, frontend, and database components together
- one app-level recipe manifest can describe a fuller deployment shape cleanly
- the pattern is believable for a small real app, not just a pedagogical pair of units

This is the point where the package becomes a realistic worked example, not only a teaching scaffold.

### `gpu-eks-h100-training/`

This proves:

- the same ordered clone-chain model can express non-app dimensions like platform, accelerator, OS, and intent
- a recipe manifest can describe a GPU-flavored deployment shape without needing a new backend type
- the AICR-style story is believable in ConfigHub terms, not only in abstract analysis

This is the point where the package stops being only about `global-app` and starts showing the broader recipe model.

## What Is Still Missing

### 1. GPU dimensions are now present, but still only at single-component scope

The package now shows:

- cloud or platform
- accelerator
- OS
- workload intent

But only in one single-component GPU example. A later multi-component GPU or platform example is still useful.

### 2. Bundle publication is still mostly a hint

The examples explain the bundle role and target association, but they do not yet walk through publishing and inspecting a real bundle end to end.

### 3. Real preflight and connection clarity are still thin

The examples do not yet show the full "what am I connected to and is it fresh" story for:

- worker
- target
- cluster
- GitOps controller if delegated
- resulting bundle

### 4. No phased validation story

The examples verify config shape, but they do not yet show:

- snapshot
- readiness validate
- deploy
- post-deploy health
- conformance

That is the main reason the later `Run` idea still matters.

### 5. No GUI-led walkthrough yet

The examples are stronger on the CLI side than the GUI side. They still need explicit GUI checkpoints.

## Recommended Next Steps

### 1. Keep all three examples

Do not replace one with another. The staged set is useful.

### 2. Add one package-level README and test story

Claude and other contributors should be able to understand:

- what this package is
- where the spec is
- which example to start with
- how to test it

### 3. Strengthen the optional target path

Add stronger preflight checks for:

- authenticated context
- worker selection or creation
- target selection
- freshness and connection clarity

### 4. Add one delegated deploy path

After direct apply is solid, add one GitOps-flavored path so the package can show:

- worker publishes bundle
- ArgoCD or Flux reconciles it
- ConfigHub still governs desired state and provenance

### 5. Add a larger multi-component GPU-style example later

The package now has a first GPU proof in:

- `eks + h100 + ubuntu + training`

The next GPU step should not be a different model. It should be a larger example that reuses the same conceptual standard across more than one component.

## Recommendation

Keep `examples/incubator/global-app-layer/` as the canonical incubator package for recipes and layers.

Use it in this order:

1. `single-component/`
2. `frontend-postgres/`
3. `realistic-app/`
4. `gpu-eks-h100-training/`
5. later, a larger multi-component GPU-style example

## Bottom Line

The package now matches the proposed spec well enough to serve as the first coherent recipe-and-layer teaching set.

The next step is not redesign. The next step is to make the package easier to run, easier to connect to real targets, and easier to understand through GUI plus CLI together.
