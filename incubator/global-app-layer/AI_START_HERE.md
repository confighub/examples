# AI Start Here

Use this page when you want to drive the `global-app-layer` package safely with Codex, Claude, Cursor, or another AI assistant.

## What This Package Is For

This package demonstrates how ConfigHub can represent layered recipes as real versioned config objects.

It is for:
- multi-component app stacks
- NVIDIA AICR-style layered recipes
- safe updates and downstream propagation
- optional direct or delegated delivery after the recipe is materialized

It is **not** only about live deployment. A large part of the value is visible in the ConfigHub database before anything is applied to a cluster.

## Safe First Steps

Start with read-only commands only:

```bash
cd incubator/global-app-layer
./find-runs.sh --json | jq

cd realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

What these do not mutate:
- they do not create spaces
- they do not create units
- they do not bind targets
- they do not apply to a cluster

## Capability Branching

### A. Docs and repo only

Use this if the human wants explanation only.

Safe path:

```bash
cd incubator/global-app-layer
./find-runs.sh --json | jq
cd realistic-app
./setup.sh --explain-json | jq
```

### B. ConfigHub database only

Use this if auth works but there is no worker or target available.

Safe path:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh
./verify.sh
```

This writes ConfigHub spaces and units, but does not deploy anything live.

### C. Live target available

Use this only when the user has a real target and worker available.

Safe path:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh <prefix> <space/target>
./verify.sh
```

Then approve and apply the deployment units explicitly.

## Exact Commands To Run

Recommended first walkthrough:

```bash
cd incubator/global-app-layer/realistic-app

# Preview the full plan first
./setup.sh --explain
./setup.sh --explain-json | jq

# Materialize the layered recipe in ConfigHub
./setup.sh

# Verify spaces, units, chain, and recipe manifest
./verify.sh
```

If the user wants the smallest example instead:

```bash
cd incubator/global-app-layer/single-component
./setup.sh --explain
./setup.sh
./verify.sh
```

If the user specifically wants the NVIDIA-shaped example:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh --explain
./setup.sh
./verify.sh
```

## What Mutates What

| Command | Reads | Writes |
|---|---|---|
| `./find-runs.sh --json` | ConfigHub labels on spaces/units | nothing |
| `./setup.sh --explain-json` | local scripts and source manifests | nothing |
| `./setup.sh` | local source manifests + current ConfigHub org | ConfigHub spaces, units, links, recipe manifest, local `.state/` |
| `./verify.sh` | ConfigHub objects created by the example | nothing |
| `./set-target.sh <space/target>` | ConfigHub deployment units, target ref | ConfigHub target bindings |
| `cub unit apply ...` | ConfigHub deployment units + target + worker | live target state |

## What Success Looks Like

After a successful ConfigHub-only run you should see:
- five or six new spaces with a shared prefix
- units for each layer
- one recipe manifest in the recipe space
- `verify.sh` passing

After a successful live path you should also see:
- targets bound on deployment units
- successful `cub unit apply`
- live resources or delegated delivery objects visible

## GUI Checkpoints

Use the GUI while you go:

1. open the new spaces created by the example prefix
2. inspect the recipe space
3. inspect one deployment unit
4. inspect the recipe manifest unit
5. if using a live target, inspect the deployment space again after apply

## Cleanup

Use the example-specific cleanup script:

```bash
./cleanup.sh
```

This removes the ConfigHub objects created by the example.
It does not magically clean unrelated cluster resources if you applied and then changed things outside the example flow.
