# `single-component`

This worked example turns one `global-app` component into an explicit layered recipe chain.

It demonstrates the model:

- `variant` = a unit specialized from an earlier unit
- `clone link` = the ConfigHub mechanism that keeps it connected upstream
- `bundle` = publish the resolved deployment output from a target

The recipe is the ordered chain of variants, not the bundle.

## Stack And Scenario

This example is for:
- ConfigHub-managed Kubernetes manifests
- the smallest layered recipe walkthrough in this package
- one backend service plus one deploy-time stub dependency

## What You Need Installed

- `cub` in `PATH`
- an authenticated ConfigHub CLI context for any mutating step
- `jq` for the JSON preview path
- optional: a live target only if you want to bind and apply

## What This Reads And Writes

What it reads:
- `../../../global-app/baseconfig/backend.yaml`
- `./postgres-stub.yaml`
- current ConfigHub context and optional target ref

What it writes:
- five ConfigHub spaces with a shared prefix
- one layered backend chain
- one deploy-stage `postgres-stub`
- one recipe manifest unit
- optional target bindings
- optional live deployment state only if you explicitly bind and apply

## What You Should Expect To See

In ConfigHub-only mode:
- five spaces sharing one prefix
- one layered backend chain
- one deploy-stage stub
- one recipe manifest unit
- `verify.sh` passing

In live mode:
- deployment units bound to a target
- successful `cub unit apply`
- live resources visible in the chosen target path

## AI-Safe Path

If you want to use this example with an AI assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## What It Builds

One component from `global-app`:

- source manifest: `../../../global-app/baseconfig/backend.yaml`

One materialized chain:

```mermaid
flowchart LR
  Base["backend-base"] --> Region["backend-us"]
  Region --> Role["backend-us-staging"]
  Role --> Recipe["backend-recipe-us-staging"]
  Recipe --> Deploy["backend-cluster-a"]
  Deploy --> Bundle["target bundle / OCI"]
```

The chain is split across five spaces:

- `catalog-base`
- `catalog-us`
- `catalog-us-staging`
- `recipe-us-staging`
- `deploy-cluster-a`

The example also writes an explicit recipe manifest unit into the recipe space. ConfigHub does not need a first-class `Recipe` type for the chain to work. The variant chain is what ConfigHub executes; the recipe manifest is the receipt that explains how it was assembled.

The recipe source now has two forms:

- [recipe.base.yaml](./recipe.base.yaml): placeholder-based base recipe, analogous to base config units that still need environment-specific values filled in
- `.state/recipe-us-staging.rendered.yaml`: rendered concrete recipe instance for this chain

The setup scripts render the concrete recipe instance from the placeholder-based base recipe.

## Layer Semantics

- `base`: original `global-app` backend manifest
- `region`: set `REGION=US` and a regional hostname
- `role`: set `ROLE=staging`, `replicas=2`, and `LOG_LEVEL=info`
- `recipe`: stamp a resolved recipe-specific chat title
- `deployment`: set namespace, cluster-local hostname, and cluster env var

## Quick Start

```bash
cd incubator/global-app-layer/single-component

# Inspect the full plan without mutating ConfigHub
./setup.sh --explain

# Machine-readable plan for AI or tooling
./setup.sh --explain-json | jq

# Ready for a fresh run
./setup.sh                              # ConfigHub-only
./setup.sh <prefix> <space/target>     # with live target
./verify.sh
```

After `./setup.sh`, prefer the printed clickable GUI URLs and `.logs/*.latest.log` files over terminal scrollback alone.

## Upgrade Flow

This is the important part of the example: upgrades move down the chain without flattening the layers.

```bash
# Update the base image tag, then push upgrades stage by stage
./upgrade-chain.sh 1.1.8

# Verify the chain still has its layer-specific mutations
./verify.sh
```

## Optional Target + Bundle Story

If you did not pass a target during setup:

```bash
./set-target.sh <space/target>
```

Then you can use normal ConfigHub apply flow on the deployment unit:

```bash
cub unit approve --space <prefix>-deploy-cluster-a backend-cluster-a
cub unit apply --space <prefix>-deploy-cluster-a backend-cluster-a
```

The bundle belongs to the target. The recipe manifest records the chain that produced the deployment, and includes a bundle hint once a target is set.

## Inspecting the Result

```bash
# Show the deployment data
cub unit get --space <prefix>-deploy-cluster-a --data-only backend-cluster-a

# Show the explicit recipe manifest
cub unit get --space <prefix>-recipe-us-staging --data-only recipe-us-staging

# Show variant ancestry (implemented with clone links)
cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-single'"
```

## Cleanup

```bash
./cleanup.sh
```

## Why This Example Exists

This is the first worked example in the `global-app-layer` package, and a worked answer to the question:

- do we need a first-class recipe object?

For now, the answer is:

- execution can stay implicit in variants + clone links
- teaching and provenance should be explicit in metadata

That is why this example uses both:

- real variant-chain units for execution
- one explicit recipe manifest unit for explanation and review
- one placeholder-based base recipe file to show the source shape before values are materialized
