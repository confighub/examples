# Import From Bundle

This stable example adapts the `import-from-bundle` flow from `cub-scout` into the official `examples` repo.

It shows an offline import path:

- read an existing debug bundle
- generate a dry-run import proposal
- inspect the namespaces, workloads, proposal, and evidence
- no live cluster required

## What This Example Is For

Use this example when you want to show import proposal generation without cluster access.

This is useful when the operator already has a debug bundle and wants a dry-run proposal before touching a live environment.

## Source

This example is adapted from:

- [cub-scout import-from-bundle](https://github.com/confighub/cub-scout/tree/main/examples/import-from-bundle)

## What It Reads

It reads:

- the copied bundle fixture under `fixtures/bundle/dev/`
- the copied expected output under `expected-output/`
- the `cub-scout` binary

## What It Writes

It writes local files only:

- `sample-output/suggestion.json`

It does not mutate ConfigHub state.
It does not mutate live infrastructure.

## Read-Only First

```bash
cd import-from-bundle
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

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It compares local JSON output against the committed expected output.

`./cleanup.sh` removes local sample output only.

## What Success Looks Like

You should get one dry-run proposal with:

- `evidence.source == "bundle"`
- one namespace: `api`
- one workload: `Deployment/api/api-server`
- one proposal unit: `api`
- one `cluster-only` workload grouping

## Evidence To Check

```bash
jq '.' sample-output/suggestion.json
jq '.evidence' sample-output/suggestion.json
jq '.proposal' sample-output/suggestion.json
jq '.workloads' sample-output/suggestion.json
```

## Why This Example Matters

This gives the stable example set a clean offline import example.

It fits between the fixture-first compare examples and the live import examples:

- no cluster required
- still import-oriented
- still evidence-first
- still honest about mutation boundaries

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
