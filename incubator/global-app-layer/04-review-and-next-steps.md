# Status and Open Gaps for `global-app-layer`

This note is for contributors who want a quick view of what is already working in the `global-app-layer` package and what still needs product or example work.

## Current Status

The package is now a coherent teaching and demo set for layered recipes in ConfigHub.

It has:

- a minimal ConfigHub first step: [`00-config-hub-hello-world.md`](./00-config-hub-hello-world.md)
- a simple one-component proof: [`single-component`](./single-component/)
- a small multi-component app proof: [`frontend-postgres`](./frontend-postgres/)
- a more realistic app proof: [`realistic-app`](./realistic-app/)
- a domain-shaped GPU recipe proof: [`gpu-eks-h100-training`](./gpu-eks-h100-training/)
- a practical explanation of ConfigHub's value on top of AICR: [confighub-aicr-value-add.md](./confighub-aicr-value-add.md)
- a package-owned e2e layer: [e2e/](./e2e/)

That is enough to teach the core model and to run believable demos.

## What Is Working Well

### 1. The recipe model is clear

The examples consistently show:

- ordered variant chains, implemented with clone links
- explicit layer meaning
- explicit recipe manifests for provenance
- deployment layers separated from shared recipe layers

### 2. Upgrades are believable

The package now gives a credible story for:

- shared updates
- downstream propagation
- preserved deployment-local values

### 3. Delivery is no longer hand-wavy

The package now has one place for:

- direct delivery
- Argo-oriented delivery
- brownfield import
- bridge flows between import and layering

### 4. The package is easier to explain

The combination of:

- [README.md](./README.md)
- [how-it-works.md](./how-it-works.md)
- [confighub-aicr-value-add.md](./confighub-aicr-value-add.md)

is now much stronger than the older scattered explanation.

## What Is Still Missing

### 1. Preflight and connection clarity in the examples

The examples still assume the user can sort out:

- authentication
- which worker is active
- which target to use
- whether a cluster is actually fresh and connected

This is still thinner than it should be for first-time live demos.

### 2. Full bundle publication as a first-class story

The examples explain the role of bundles and targets, but the package still does not present a strong end-to-end bundle publication and inspection walkthrough.

That follow-up should also add a short explainer for how NVIDIA AICR talks about bundles, integrity, and attestation, using the NVIDIA blog post [Validate Kubernetes for GPU Infrastructure with Layered, Reproducible Recipes](https://developer.nvidia.com/blog/validate-kubernetes-for-gpu-infrastructure-with-layered-reproducible-recipes/) as a reference point. In particular, explain:

- what the generated bundle contains
- how integrity checksums fit in
- what signed SBOMs and image attestations mean in practice
- how that maps to what ConfigHub should show or preserve in a future bundle story

### 3. A stronger GUI-led walkthrough

The new value-add guide includes GUI steps, but the package still does not yet have one dedicated GUI-first walkthrough for users who want to understand the chain visually before using the CLI deeply.

### 4. A phased operational story

The examples verify config shape and cluster results, but they still do not yet show a formal operational flow like:

- snapshot
- readiness validate
- deploy
- post-deploy health
- conformance

That remains the strongest reason to keep exploring a better `cub-proc` or `Operation` story.

### 5. A larger fleet-style example

The package now demonstrates the idea of variants and preserved local overrides, but it does not yet have a fuller fleet example where many similar deployments differ by one or two meaningful operational parameters.

## Recommended Next Steps

If someone is continuing this work, the most useful next moves are:

1. add stronger preflight and live-target clarity to the examples
2. add one GUI-first walkthrough page for the main demo paths
3. add one stronger fleet or multi-variant example
4. connect the package more directly to a future `cub-proc` or `Operation` workflow once that direction firms up

## Bottom Line

Yes, this package is now worth using as the main incubator home for recipes and layers.

What remains is no longer basic model design.
What remains is product polish:

- clearer live connections
- clearer GUI guidance
- clearer operational flow
- larger real-world variant/fleet stories
