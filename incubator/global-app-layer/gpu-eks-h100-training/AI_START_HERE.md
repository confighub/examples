# AI Start Here

Use this page when you want to drive `gpu-eks-h100-training` safely with an AI assistant.

## What This Example Is For

This example demonstrates an NVIDIA-shaped layered recipe in ConfigHub with:
- `gpu-operator`
- `nvidia-device-plugin`

It is the clearest AICR-shaped example in the package.

## WET-First, Not Live-First

This example starts by materializing intended state in ConfigHub.

The normal path is:
1. preview with `./setup.sh --explain`
2. materialize with `./setup.sh`
3. verify with `./verify.sh`
4. optionally bind a target
5. optionally apply live

So `setup.sh` is ConfigHub-first, not cluster-first.

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
- if a real target is visible, run `../preflight-live.sh <space/target>` before you offer the live path
- only use the live path when preflight reports `applyReady: true`

For this example, there is one more gate:
- the honest live proof today is the direct `Kubernetes` target
- if preflight says `providerType: "ArgoCDRenderer"`, stop and note that this example materializes raw Kubernetes deployment units, while the current renderer path expects Argo CD `Application` payloads and does not serve as the final Argo-sync proof

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

If the human wants the full lifecycle after setup + verify, continue with:

- [../whole-journey.md](../whole-journey.md)

Use the same live target, upgrade, and downstream-variant pattern there.
For functional NVIDIA proof, also replace the stub images and use GPU-capable nodes.

## Ready For A Fresh Run

```bash
./setup.sh                              # ConfigHub-only
./setup.sh <prefix> <space/target>     # with live target
./verify.sh
```

If you start ConfigHub-only and later want the live path:

```bash
./set-target.sh <space/target>
```

The helper now rejects `ArgoCDRenderer` targets for this example before mutating deployment units.

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

The easiest path is to open the clickable URLs printed by `./setup.sh`.

## CLI Footguns To Avoid

- use `cub version`, not `cub --version`
- use `cub context list`, not `cub context current`
- use the jq anchors in `contracts.md` for machine-readable unit inspection

## What Mutates What

| Command | Writes |
|---|---|
| `./setup.sh --explain-json` | nothing |
| `./setup.sh` | ConfigHub spaces, units, links, recipe manifest, local `.state/`, local `.logs/setup.latest.log` |
| `./verify.sh` | local `.logs/verify.latest.log` |
| `./set-target.sh <space/target>` | ConfigHub target bindings, local `.logs/set-target.latest.log` |
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
