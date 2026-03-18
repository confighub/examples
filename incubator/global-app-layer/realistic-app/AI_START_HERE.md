# AI Start Here

Use this page when you want to drive `realistic-app` safely with an AI assistant.

## What This Example Is For

This example demonstrates a layered small app recipe in ConfigHub:
- `backend`
- `frontend`
- `postgres`

It is the clearest app-shaped example in `global-app-layer`.

## What You Need Installed

- `cub` in `PATH`
- an authenticated ConfigHub CLI context for any mutating step
- `jq` for the JSON preview path
- optional: a live target only if you want to bind and apply

## Safe First Steps

Start read-only:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

These do not mutate ConfigHub or a cluster.

## Capability Branching

### A. Docs / preview only

Use the explain modes only.

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
- five new spaces with one shared prefix
- three layered chains
- one recipe manifest unit
- `verify.sh` passing

In live mode:
- deployment units bound to a target
- successful `cub unit apply`
- live resources appearing in the chosen target path
