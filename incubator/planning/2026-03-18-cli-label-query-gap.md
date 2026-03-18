# CLI Label Query Gap

**Status**: backlog
**Filed**: 2026-03-18
**Context**: discovered while testing global-app-layer examples with multiple agents

## Problem

The `cub` CLI lets you set labels on spaces and units:

```bash
cub space create my-space --label "ExampleName=global-app-layer-realistic-app"
cub unit create --space my-space my-unit file.yaml --label "Component=backend"
```

But there is no way to query by label:

```bash
# This does not exist
cub space list --label "ExampleName=global-app-layer-realistic-app"
```

Labels are write-only from the CLI perspective.

## Impact

1. **Multi-agent coordination fails.** Agent A creates spaces with labels. Agent B cannot find them by label — only by prefix/name pattern matching with `grep`.

2. **Cleanup relies on naming conventions, not semantics.** The `cleanup.sh` scripts use `cub space delete --where "Labels.ExampleChain = 'prefix'"` (which works via the bulk delete API), but `cub space list` has no equivalent filter. You can delete by label but not list by label.

3. **"What's running right now?" requires heuristics.** To find all global-app-layer spaces, you have to `cub space list | grep -E '(catalog-base|catalog-us|recipe-|deploy-)'` instead of querying by `ExampleName`.

4. **The label convention in the examples is systematically applied but unverifiable.** Every space and unit gets `ExampleName`, `ExampleChain`, `Recipe`, `Component`, and `Layer` labels. These labels exist in the database but are invisible to CLI users except through `cub space get` or `cub unit get` on individual objects.

## Current Labels Set by Examples

```
--label "ExampleName=${EXAMPLE_NAME}"
--label "ExampleChain=$(state_prefix)"
--label "Recipe=${CHAIN_LABEL}"
--label "Component=${COMPONENT}"
--label "Layer=${layer}"
```

## Proposed Fix

Add label filtering to list commands:

```bash
cub space list --label "ExampleName=global-app-layer-realistic-app"
cub unit list --label "Component=backend"
cub space list --label "ExampleChain=hug-hug"
```

This would make labels queryable and close the gap between the bulk delete API (which already supports label-based `--where`) and the list commands (which do not).

## Workarounds

Until the CLI supports label filtering:

- Use prefix-based `grep` on `cub space list` output
- Use `cub space delete --where "Labels.X = 'Y'"` for cleanup (this works today)
- Use the ConfigHub GUI for label-based browsing
