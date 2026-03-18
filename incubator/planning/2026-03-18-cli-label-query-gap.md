# Label Query UX Gap

**Status**: backlog
**Filed**: 2026-03-18
**Context**: discovered while testing `global-app-layer` examples with multiple agents

## Problem

The core CLI capability already exists:

```bash
cub space list --where "Labels.ExampleName = 'global-app-layer-realistic-app'" --json
cub space list --where "Labels.ExampleChain = 'hug-hug'" --json
cub space list --where "Labels.ExampleName LIKE 'global-app-layer-%'" --names
```

But this is not obvious to new users or AI assistants. People naturally guess:

```bash
# This does not exist today
cub space list --label "ExampleName=global-app-layer-realistic-app"
```

So the gap is not "labels are unqueryable". The gap is that **label-based discovery is too hard to discover and too easy to guess wrong**.

## Impact

1. **Multi-agent coordination is brittle.** Agent A creates labeled spaces. Agent B may not know the prefix and may guess the wrong CLI shape instead of using `--where "Labels.X = 'Y'"`.

2. **Example handoff is harder than it should be.** The `global-app-layer` examples set `ExampleName`, `ExampleChain`, `Recipe`, `Component`, and `Layer` labels consistently, but a second agent has to know both the labels and the `--where` syntax to find them.

3. **The common mental model is wrong.** People think:
   - `--label` for create/update
   - therefore probably `--label` for list/filter

4. **Docs and examples have to carry extra explanation.** The `global-app-layer` package now ships `find-runs.sh` mainly to bridge this discovery gap.

## Current Labels Set by Examples

```bash
--label "ExampleName=${EXAMPLE_NAME}"
--label "ExampleChain=$(state_prefix)"
--label "Recipe=${CHAIN_LABEL}"
--label "Component=${COMPONENT}"
--label "Layer=${layer}"
```

These labels are queryable today through `--where`.

## Proposed Fixes

### Minimum fix: improve discoverability

Make label-query examples much more obvious in CLI help and docs:

```bash
cub space list --where "Labels.ExampleName = 'global-app-layer-realistic-app'" --json
cub unit list --where "Labels.Component = 'backend'" --json
```

This should appear in:

- `cub space list --help`
- `cub unit list --help`
- agent-oriented CLI docs

### Optional sugar: add `--label` shorthand for list commands

Support:

```bash
cub space list --label "ExampleName=global-app-layer-realistic-app"
cub unit list --label "Component=backend"
```

Implemented as shorthand for the equivalent `--where "Labels.X = 'Y'"` expression.

This is a UX improvement, not a missing backend capability.

## What This Is Not

- not a claim that labels are write-only
- not a claim that list commands cannot filter by labels
- not a blocker for the examples package, which can already query labels with `--where`

## Workarounds Available Today

- use `--where "Labels.X = 'Y'"` on `cub space list`
- use `--where "Labels.X = 'Y'"` on `cub unit list`
- use [`global-app-layer/find-runs.sh`](../global-app-layer/find-runs.sh) for example-run discovery
