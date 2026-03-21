# AI Start Here

Use this page when you want to drive `gpu-eks-h100-training` safely with an AI assistant.

## What This Example Is For

This example demonstrates an NVIDIA-shaped layered recipe in ConfigHub with:
- `gpu-operator`
- `nvidia-device-plugin`

It is the clearest AICR-shaped example in the package.

## WET-First, Then Deployment Variants

This example starts by materializing intended state in ConfigHub.

The normal path is:
1. preview with `./setup.sh --explain`
2. materialize with `./setup.sh`
3. verify with `./verify.sh`
4. optionally bind one or both deployment variants to targets
5. optionally apply live

The shared recipe is the app-level intent. The deployment variants sit at the leaf:
- direct deployment variant
- Flux deployment variant

## What You Need Installed

- `cub` in `PATH`
- an authenticated ConfigHub CLI context for any mutating step
- `jq` for the JSON preview path
- optional: compatible live targets only if you want to bind and apply
- optional: GPU-capable nodes and real images only if you want functional NVIDIA proof rather than structural proof

## Capability Check

Check capability before mutating anything:

```bash
which cub
cub version
cub context list --json | jq
cub target list --space "*" --json | jq
```

Use this rule:
- if `cub` is missing or auth is unavailable, stop at preview mode
- if auth works but there is no relevant target, use ConfigHub-only mode
- if a real target is visible, run `../preflight-live.sh <space/target>` before you offer the live path
- only use the live path when preflight reports `applyReady: true`

For this example, route by provider type:
- `Kubernetes` -> direct deployment variant
- `FluxOCI` or `FluxOCIWriter` -> Flux deployment variant
- `ArgoCDRenderer` and `FluxRenderer` are not deployment targets for this example and should be rejected

## Important Note

This example is a structural proof:
- the layer shape is real
- the images are stubbed by default
- real NVIDIA deployment requires real images and GPU-capable nodes

## Safe First Steps

Start read-only:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh --explain
./setup.sh --explain-json | jq
```

These do not mutate ConfigHub or a cluster.

After `./setup.sh`, use:
- the printed clickable GUI URLs
- `.logs/setup.latest.log`
- `.logs/set-target.latest.log`
- `.logs/verify.latest.log`

instead of relying on terminal scrollback alone.

For the live branch, do not rely on target visibility alone.
From this directory, run:

```bash
../preflight-live.sh <space/target>
../preflight-live.sh <space/target> --json | jq
```

Only call the live path ready if preflight reports `applyReady: true`.

## Ready For A Fresh Run

```bash
./setup.sh                                            # ConfigHub-only
./setup.sh <prefix> <kubernetes-target>               # bind direct variant during setup
./setup.sh <prefix> <kubernetes-target> <fluxoci-target>  # bind direct and Flux variants during setup
./verify.sh
```

If you start ConfigHub-only and later want the live path:

```bash
./set-target.sh <kubernetes-target>
./set-target.sh <fluxoci-target>
```

The helper routes targets by provider type and only binds the compatible deployment variant.

## Capability Branching

### A. Docs / preview only

Use the explain modes only. This is also the right stop point if auth is missing.

### B. ConfigHub-only mode

Use:

```bash
./setup.sh
./verify.sh
```

This writes the layered GPU recipe and both deployment variants into ConfigHub, but does not deploy anything live.

### C. Live target mode

Use:

```bash
./setup.sh <prefix> <kubernetes-target>
./set-target.sh <fluxoci-target>   # optional second branch
./verify.sh
```

Then approve and apply the deployment units explicitly for the variant you want to prove.

## Verification Modes

- Preview only:
  - `./setup.sh --explain`
  - `./setup.sh --explain-json | jq`
- ConfigHub-only:
  - `./setup.sh`
  - `./verify.sh`
- Live target:
  - `./setup.sh <prefix> <kubernetes-target>`
  - `./set-target.sh <fluxoci-target>` if you want the Flux branch too
  - `./verify.sh`
  - explicit `cub unit apply ...`

## GUI Checkpoints

As you go, inspect these in the ConfigHub GUI:

1. `<prefix>-recipe-eks-h100-ubuntu-training`
   - inspect `recipe-eks-h100-ubuntu-training-stack`
2. `<prefix>-deploy-cluster-a`
   - inspect `gpu-operator-cluster-a`
3. `<prefix>-deploy-cluster-a-flux`
   - inspect `gpu-operator-cluster-a-flux`
4. compare the recipe manifest and the two deployment variants
   - confirm the recipe receipt exists
   - confirm both deployment variants exist
5. if targets are set
   - inspect the matching deployment variant again and confirm the target binding is visible
6. if you apply live
   - inspect the deployment space after apply and compare intended state vs live result

The easiest path is to open the clickable URLs printed by `./setup.sh`.

## CLI Footguns To Avoid

- use `cub version`, not `cub --version`
- use `cub context list`, not `cub context current`
- use the jq anchors in `contracts.md` for machine-readable unit inspection
- do not treat `FluxRenderer` as the deployment target for this example; it is the import-and-render path for existing Flux resources

## What Mutates What

| Command | Writes |
|---|---|
| `./setup.sh --explain-json` | nothing |
| `./setup.sh` | ConfigHub spaces, units, links, recipe manifest, local `.state/`, local `.logs/setup.latest.log` |
| `./verify.sh` | local `.logs/verify.latest.log` |
| `./set-target.sh <target> ...` | ConfigHub target bindings for compatible deployment variants, local `.logs/set-target.latest.log` |
| `cub unit apply ...` | live target state |

## What Success Looks Like

In ConfigHub-only mode:
- seven new spaces with one shared prefix
- two layered GPU chains
- two deployment variants at the leaf
- one stack-level recipe manifest unit
- `verify.sh` passing

In live mode:
- direct and or Flux deployment variants bound to compatible targets
- successful `cub unit apply`
- live resources or delegated delivery objects visible
