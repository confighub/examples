# Fleet Import

This incubator example adapts the `fleet-import` flow from `cub-scout` into the official `examples` repo.

It shows one multi-cluster follow-on path after per-cluster import:

- take two existing import JSON files
- aggregate them into one fleet view
- generate one unified proposal
- no live cluster required

## What This Example Is For

Use this example when you want to show how per-cluster import results can be merged into one multi-cluster proposal.

This is a read-only aggregation example. It does not mutate ConfigHub or live infrastructure.

## Source

This example is adapted from:

- [cub-scout fleet-import](https://github.com/confighub/cub-scout/tree/main/examples/fleet-import)

## What It Reads

It reads:

- `cluster-dev.json`
- `cluster-prod.json`
- the copied expected output under `expected-output/`
- the `cub-scout` binary

## What It Writes

It writes local files only:

- `sample-output/fleet-summary.json`

It does not mutate ConfigHub state.
It does not mutate live infrastructure.

## Read-Only First

```bash
cd incubator/fleet-import
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Quick Start

```bash
./setup.sh
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` does not mutate ConfigHub or live infrastructure. It writes local output only.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It compares local output against the committed expected output.

`./cleanup.sh` removes local sample output only.

## What Success Looks Like

You should get one unified fleet output with:

- `summary.totalClusters == 2`
- `summary.totalWorkloads == 7`
- `proposal.appSpace == "fleet-team"`
- one proposal that groups dev and prod variants under shared apps

## Evidence To Check

```bash
jq '.summary' sample-output/fleet-summary.json
jq '.proposal' sample-output/fleet-summary.json
jq '.summary.byApp | to_entries[] | select(.value | length > 1)' sample-output/fleet-summary.json
```

## Why This Example Matters

This extends the import story from one cluster to many.

It is useful when the human wants to show:

- aggregation of existing import facts
- one fleet-level proposal
- one place to see which apps span multiple clusters

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
