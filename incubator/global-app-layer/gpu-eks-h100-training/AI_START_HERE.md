# AI Start Here

Use this page when you want to drive `gpu-eks-h100-training` safely with an AI assistant.

## What This Example Is For

This example demonstrates an NVIDIA-shaped layered recipe in ConfigHub with:
- `gpu-operator`
- `nvidia-device-plugin`

It is the clearest AICR-shaped example in the package.

## What You Need Installed

- `cub` in `PATH`
- an authenticated ConfigHub CLI context for any mutating step
- `jq` for the JSON preview path
- optional: a live target only if you want to bind and apply
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
- only use the live path when a real target exists

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

## Capability Branching

### A. Docs / preview only

Use the explain modes only. This is also the right stop point if auth is missing.

### B. ConfigHub-only mode

Use:

```bash
./setup.sh
./verify.sh
```

This writes the layered GPU recipe into ConfigHub, but does not deploy anything live.

### C. Live target mode

Use:

```bash
./setup.sh <prefix> <space/target>
./verify.sh
```

Then approve and apply the deployment units explicitly.

## Verification Modes

- Preview only:
  - `./setup.sh --explain`
  - `./setup.sh --explain-json | jq`
- ConfigHub-only:
  - `./setup.sh`
  - `./verify.sh`
- Live target:
  - `./setup.sh <prefix> <space/target>`
  - `./verify.sh`
  - explicit `cub unit apply ...`

## GUI Checkpoints

As you go, inspect these in the ConfigHub GUI:

1. `<prefix>-recipe-eks-h100-ubuntu-training`
   - inspect `recipe-eks-h100-ubuntu-training-stack`
2. `<prefix>-deploy-cluster-a`
   - inspect `gpu-operator-cluster-a`
3. compare the recipe manifest and one deployment unit
   - confirm the recipe receipt exists
   - confirm the deployment variant exists
4. if a target is set
   - inspect `gpu-operator-cluster-a` again and confirm the target binding is visible
5. if you apply live
   - inspect the deployment space after apply and compare intended state vs live result

## CLI Footguns To Avoid

- use `cub version`, not `cub --version`
- use `cub context list`, not `cub context current`
- use the jq anchors in `contracts.md` for machine-readable unit inspection

## What Mutates What

| Command | Writes |
|---|---|
| `./setup.sh --explain-json` | nothing |
| `./setup.sh` | ConfigHub spaces, units, links, recipe manifest, local `.state/` |
| `./verify.sh` | nothing |
| `./set-target.sh <space/target>` | ConfigHub target bindings |
| `cub unit apply ...` | live target state |

## What Success Looks Like

In ConfigHub-only mode:
- six new spaces with one shared prefix
- two layered GPU chains
- one stack-level recipe manifest unit
- `verify.sh` passing

In live mode:
- deployment units bound to a target
- successful `cub unit apply`
- live resources or delegated delivery objects visible
