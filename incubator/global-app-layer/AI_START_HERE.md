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
git rev-parse --show-toplevel
which cub
kubectl version --client 2>/dev/null || true
cub version
cub context list --json | jq

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

Once you run `./setup.sh`, use the printed artifacts instead of relying on terminal scrollback:
- clickable GUI URLs for the recipe space, deploy space, recipe manifest, and one deployment unit
- durable logs in `.logs/setup.latest.log`, `.logs/set-target.latest.log`, `.logs/verify.latest.log`, and `.logs/cleanup.latest.log`

## Ready For A Fresh Run

Use the same short path across the layered examples:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh                              # ConfigHub-only
./setup.sh <prefix> <space/target>     # with live target
./verify.sh
```

If you start ConfigHub-only and later want the live path:

```bash
./set-target.sh <space/target>
```

## Capability Check

Before you choose a path, check what is actually available:

```bash
cub version
cub context list --json | jq
cub target list --space "*" --json | jq
```

Interpret the result like this:
- if `cub` is missing or auth is unavailable, stay in preview mode
- if auth works but target listing is empty or irrelevant, use ConfigHub-only mode
- if a real target is available, you can offer the live path

## Capability Branching

### A. Preview only

Use this if the human wants explanation only, or if `cub` auth is unavailable.

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

# Ready for a fresh run
./setup.sh                              # ConfigHub-only
./setup.sh <prefix> <space/target>     # with live target
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
| `./setup.sh` | local source manifests + current ConfigHub org | ConfigHub spaces, units, links, recipe manifest, local `.state/`, local `.logs/setup.latest.log` |
| `./verify.sh` | ConfigHub objects created by the example | local `.logs/verify.latest.log` |
| `./set-target.sh <space/target>` | ConfigHub deployment units, target ref | ConfigHub target bindings, local `.logs/set-target.latest.log` |
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
2. open the recipe space and inspect the recipe manifest unit
3. open one deployment unit and inspect its upstream chain and current intended state
4. if a target is set, inspect the deployment unit again and confirm the target binding is visible
5. if using the live path, inspect the deployment space again after apply and compare intended state vs live result

The easiest way to get there is to use the clickable URLs printed by `./setup.sh`.

## CLI Footguns To Avoid

- use `cub version`, not `cub --version`
- use `cub context list`, not `cub context current`
- for machine-readable unit inspection, prefer the exact jq examples in `contracts.md`

## Cleanup

Use the example-specific cleanup script:

```bash
./cleanup.sh
```

This removes the ConfigHub objects created by the example.
It does not magically clean unrelated cluster resources if you applied and then changed things outside the example flow.
The cleanup output is also captured in `.logs/cleanup.latest.log`.
