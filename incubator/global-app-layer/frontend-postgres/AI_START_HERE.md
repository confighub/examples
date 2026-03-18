# AI Start Here

Use this page when you want to drive `frontend-postgres` safely with an AI assistant.

## What This Example Is For

This example demonstrates a layered small app recipe in ConfigHub with:
- `frontend`
- `postgres`
- one deploy-time `backend-stub`

It is the clearest two-component example in `global-app-layer`.

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
- only use the live path when a real target exists

## Safe First Steps

Start read-only:

```bash
cd incubator/global-app-layer/frontend-postgres
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

Then approve and apply the deployment units explicitly.

## Capability Branching

### A. Docs / preview only

Use the explain modes only. This is also the right stop point if auth is missing.

### B. ConfigHub-only mode

Use:

```bash
./setup.sh
./verify.sh
```

This writes spaces, units, links, the recipe manifest, and the deploy-time stub into ConfigHub, but does not deploy anything live.

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

1. `<prefix>-recipe-us-staging`
   - inspect `recipe-us-staging-app`
2. `<prefix>-deploy-cluster-a`
   - inspect `frontend-cluster-a`
3. compare the recipe manifest and one deployment unit
   - confirm the recipe receipt exists
   - confirm the deployment variant exists
4. if a target is set
   - inspect `frontend-cluster-a` again and confirm the target binding is visible
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
- five new spaces with one shared prefix
- two layered chains
- one deploy-stage `backend-stub`
- one recipe manifest unit
- `verify.sh` passing

In live mode:
- deployment units bound to a target
- successful `cub unit apply`
- live resources appearing in the chosen target path
