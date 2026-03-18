# AI Start Here

Use this page when you want to drive `realistic-app` safely with an AI assistant.

## What This Example Is For

This example demonstrates a layered small app recipe in ConfigHub:
- `backend`
- `frontend`
- `postgres`

It is the clearest app-shaped example in `global-app-layer`.

## WET-First, Not Live-First

This example starts by materializing intended state in ConfigHub.

The normal path is:
1. preview with `./setup.sh --explain`
2. materialize with `./setup.sh`
3. verify with `./verify.sh`
4. optionally bind a target
5. optionally apply live with `./apply-live.sh`

So `setup.sh` is ConfigHub-first, not cluster-first.

## What You Need Installed

- `cub` in `PATH`
- an authenticated ConfigHub CLI context for any mutating step
- `jq` for the JSON preview path
- optional: a live target only if you want to bind and apply

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

## Safe First Steps

Start read-only:

```bash
cd incubator/global-app-layer/realistic-app
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

That walkthrough covers:
- live target binding and apply
- shared upstream upgrades
- a custom downstream deployment variant

## Ready For A Fresh Run

```bash
./setup.sh                              # ConfigHub-only
./setup.sh <prefix> <space/target>     # with live target
./verify.sh
```

If you start ConfigHub-only and later want the live path:

```bash
./set-target.sh <space/target>
./apply-live.sh
```

Prefer `./apply-live.sh` over ad hoc manual approval/apply steps.
It preflights the target, refreshes the deploy clones from upstream, refreshes the recipe receipt, applies the namespace bootstrap unit first, and only then applies the app units.

## Capability Branching

### A. Docs / preview only

Use the explain modes only. This is also the right stop point if auth is missing.

### B. ConfigHub-only mode

Use:

```bash
./setup.sh
./verify.sh
```

This writes spaces, units, links, and the recipe manifest into ConfigHub, but does not deploy anything live.

### C. Live target mode

Use:

```bash
./setup.sh <prefix> <space/target>
./verify.sh
```

Then approve and apply the deployment units explicitly.
Prefer `./apply-live.sh`, which preflights the target, refreshes the deployment units from upstream, applies the namespace bootstrap unit first, and then applies the app units.

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
  - `./apply-live.sh`

## GUI Checkpoints

As you go, inspect these in the ConfigHub GUI:

1. `<prefix>-recipe-us-staging`
   - inspect `recipe-us-staging-realistic-app`
2. `<prefix>-deploy-cluster-a`
   - inspect `backend-cluster-a`
3. compare the recipe manifest and one deployment unit
   - confirm the recipe receipt exists
   - confirm the deployment variant exists
4. if a target is set
   - inspect `backend-cluster-a` again and confirm the target binding is visible
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
| `./apply-live.sh` | ConfigHub approvals, live target state, local `.logs/apply-live.latest.log` |

## What Success Looks Like

In ConfigHub-only mode:
- five new spaces with one shared prefix
- three layered chains
- one recipe manifest unit
- `verify.sh` passing

In live mode:
- deployment units bound to a target
- successful `./apply-live.sh`
- backend, frontend, and postgres all reaching `Ready` with `ApplyCompleted`
- live resources appearing in the chosen target path
