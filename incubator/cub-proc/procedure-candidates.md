# `cub-proc` Procedure Candidates From Runnable Examples

This document maps the proposed `cub-proc` idea to runnable examples in this repo.

The point is not to prove that every example should become a `cub-proc` profile. The point is to identify which bounded procedures are already visible enough, repeated enough, and important enough to justify a standard operational record later.

## Selection Rule

A good candidate procedure has most of these properties:

- more than one phase
- real ordering constraints
- assertions that matter after the main mutation step
- evidence spread across more than one system
- a meaningful handoff or interruption problem

## Best Current Candidates

### 1. `gitops-import/argo`

Example:

- [../gitops-import-argo](../gitops-import-argo/README.md)

Why it is strong:

- cluster setup, controller setup, worker setup, discovery, import, and verification are separate phases
- the happy path and the broken paths are both useful evidence
- the procedure spans ConfigHub, ArgoCD, Kubernetes, and optional `cub-scout`
- the operator must distinguish import facts from live controller facts

Likely top-level phases:

- `preflight`
- `connect-cluster`
- `discover`
- `import`
- `assert`

Likely assertions:

- discovery target is present
- Argo applications were discovered
- selected dry units rendered
- wet units were created
- at least one expected healthy app is present
- controller-side failures are surfaced rather than hidden

### 2. `gitops-import/flux`

Example:

- [../gitops-import-flux](../gitops-import-flux/README.md)

Why it is strong:

- same overall shape as the Argo path, so it is a good sibling profile
- the current example already shows one healthy path and several intentionally broken paths
- the procedure spans ConfigHub, Flux, Kubernetes, and optional `cub-scout`
- the operator must distinguish renderer success from source-readiness failures

Likely top-level phases:

- `preflight`
- `connect-cluster`
- `discover`
- `import`
- `assert`

Likely assertions:

- discovery target is present
- Flux deployers were discovered
- `podinfo` rendered successfully
- broken source paths are surfaced with explicit evidence
- wet units were created

### 3. `demo-data/install`

Example:

- [../../promotion-demo-data](../../promotion-demo-data/README.md)
- [why-cub-proc-example-promotions.md](./why-cub-proc-example-promotions.md)

Why it is strong:

- it is already a clear bounded procedure with phases
- it is large enough to benefit from a stable operational record
- it does not require a live cluster, so it is a lower-risk early profile
- it currently lacks built-in assertions and still suppresses some errors

Likely top-level phases:

- `create-infra-spaces`
- `create-app-spaces`
- `seed-dev`
- `clone-lower-envs`
- `clone-prod`
- `label`
- `customize`
- `assert`

Likely assertions:

- expected space count
- expected unit count
- label coverage
- prod resource overrides
- intentional version skew

### 4. `global-app/install`

Example:

- [../../global-app](../../global-app/README.md)

Why it is strong:

- this is one of the clearest examples of a real install procedure
- it naturally splits into config materialization, target resolution, apply, and verification
- it has a direct path today and can later express delegated variants more clearly

Likely top-level phases:

- `preflight`
- `materialize-config`
- `bind-targets`
- `apply`
- `assert`

Likely assertions:

- expected spaces and units exist
- targets resolved as expected
- apply completed where requested
- expected live services or workloads are present

### 5. `gpu-stack/install`

Example:

- [../global-app-layer/gpu-eks-h100-training](../global-app-layer/gpu-eks-h100-training/README.md)

Why it is strong:

- it already has one shared recipe and two deployment variants at the leaf
- it has a clear distinction between direct deployment and Flux deployment
- the deployment-variant model is exactly the kind of place where explicit apply mode matters

Likely top-level phases:

- `preflight`
- `materialize-recipe`
- `bind-variant-targets`
- `apply`
- `assert`

Likely assertions:

- recipe manifest exists
- direct deployment units exist
- Flux deployment units exist
- chosen variant is bound to a compatible target
- selected variant applied successfully

## What Should Not Block On `cub-proc`

The current examples do not need `cub-proc` to be useful.

The GitOps import examples already land the current wedge without it. The layered examples already explain deployment variants without it. The promotion dataset already demonstrates the App-Deployment-Target model without it.

That is good. It means `cub-proc` can be tested against real examples instead of being a prerequisite for them.

## Suggested Order For Future Procedure Profiles

If `cub-proc` work resumes, the cleanest order is:

1. `demo-data/install`
2. `gitops-import/flux`
3. `gitops-import/argo`
4. `global-app/install`
5. `gpu-stack/install`

That order starts with a lower-risk ConfigHub-only procedure, then moves into the import-and-evidence wedge, then into the richer live deployment stories.
