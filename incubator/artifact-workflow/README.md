# Artifact Workflow

This incubator example adapts the artifact workflow from `cub-scout` into the official `examples` repo.

It shows a small no-cluster offline-debug path:

- one copied debug bundle
- one `bundle inspect --format json` run
- one `bundle replay --format json` run for drift findings
- one `bundle summarize --format json` run

## What This Example Is For

Use this example when you want to show how a captured debug bundle can be inspected and replayed later without cluster access.

This example does not write ConfigHub state and does not require a cluster.

## Source

This example is adapted from:

- [cub-scout workflows](https://github.com/confighub/cub-scout/tree/main/examples/workflows)

## What It Reads

It reads:

- the copied bundle fixture under `fixtures/debug-bundle/`
- the `cub-scout` binary

## What It Writes

It writes local verification output under `sample-output/`:

- `bundle-inspect.json`
- `bundle-replay-drift.json`
- `bundle-summarize.json`

It does not write ConfigHub state and does not mutate live infrastructure.

## Read-Only First

```bash
cd incubator/artifact-workflow
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

`./setup.sh` is read-only with respect to ConfigHub and live infrastructure. It only reads the copied bundle fixture and writes local output files.

`./verify.sh` is read-only. It checks that the inspect, replay, and summarize outputs contain the expected facts.

`./cleanup.sh` only removes local sample output.

## What Success Looks Like

At the bundle-inspect level you should see:

- target `Deployment/web-frontend` in `production`
- bundle contents showing session and drift

At the replay level you should see:

- one warning drift finding for `Deployment/web-frontend`
- a replay summary with one warning finding

At the summarize level you should see:

- a structured summary payload derived from the same bundle
- Git metadata including branch `main`
- one warning risk signal and `driftCount == 1`

## Evidence To Check

```bash
./verify.sh
jq '{target, contents}' sample-output/bundle-inspect.json
jq '{summary, findings}' sample-output/bundle-replay-drift.json
jq '{target, gitContext, changes, riskSignals}' sample-output/bundle-summarize.json
```

## Why This Example Matters

This gives the incubator set a reproducible offline-debug story.

It answers a practical question quickly:

- once someone has captured a bundle, what can another person or tool do with it later without cluster access?

That makes it a good companion to the import and compare examples, because it focuses on portable captured evidence rather than live collection.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
