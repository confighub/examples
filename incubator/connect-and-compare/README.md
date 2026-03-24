# Connect And Compare

This incubator example adapts the fixture-first `connect-and-compare` flow from `cub-scout` into the official `examples` repo.

It shows one of the simplest useful stories in the current wedge:

- standalone signal from `cub-scout doctor`
- a compare view of Git intent versus observed state
- a synthetic ChangeSet history view
- no live cluster required

## What This Example Is For

Use this example when you want to show visible value in under a minute without relying on live apply or controller status.

This is a read-only evidence example. It reads fixtures and writes local output snapshots only.

## Source

This example is adapted from:

- [cub-scout connect-and-compare](https://github.com/confighub/cub-scout/tree/main/examples/connect-and-compare)

## What It Reads

It reads:

- local doctor fixture input
- a local Git repo fixture
- a local observed bundle fixture
- a local synthetic ChangeSet history fixture
- the `cub-scout` binary

## What It Writes

It writes local files only:

- `sample-output/01-doctor.txt`
- `sample-output/02-connect.txt`
- `sample-output/03-compare.json`
- `sample-output/04-history.txt`

It does not mutate ConfigHub state.
It does not mutate live infrastructure.

## Read-Only First

```bash
cd incubator/connect-and-compare
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Quick Start

Generate local evidence snapshots:

```bash
./setup.sh
```

Verify against committed expected output:

```bash
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` does not mutate ConfigHub or live infrastructure. It writes local sample output only.

`./verify.sh` does not mutate ConfigHub or live infrastructure. It writes only to a temporary directory.

`./cleanup.sh` removes local sample output only.

## What Success Looks Like

You should end up with four artifacts that show a coherent operator story:

- doctor output with immediate standalone signal
- a connect step placeholder
- compare JSON showing aligned, git-only, and cluster-only findings
- history output showing who changed what from ChangeSets

The compare result should surface a real mixed state:

- aligned workloads
- one Git-only app
- one cluster-only workload

## Evidence To Check

```bash
jq '.alignment' sample-output/03-compare.json
jq '.alignment[] | select(.status != "aligned")' sample-output/03-compare.json
cat sample-output/01-doctor.txt
cat sample-output/04-history.txt
```

This example is useful because it shows the evidence model directly:

- standalone signal
- compare output
- history output

without needing live infrastructure.

## Why This Example Matters

This is a good bridge between the import-first examples and the app-style examples.

It keeps the current product story tight:

- organize evidence
- compare intent and observed state
- inspect history
- do not overclaim runtime authority

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
