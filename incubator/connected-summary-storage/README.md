# Connected Summary Storage

This incubator example adapts the `connected-summary-storage` flow from `cub-scout` into the official `examples` repo.

It shows a small no-cluster reporting path:

- one copied connected-summary store
- one `summary list --json` query over all records
- one filtered `summary list --json` query for one cluster and namespace
- one `summary slack --dry-run` digest built from the same stored records

## What This Example Is For

Use this example when you want to show how persisted connected scan and GitOps status summaries can be queried and turned into automation-friendly digests after collection has already happened.

This example does not write ConfigHub state and does not require a cluster.

## Source

This example is adapted from:

- [cub-scout connected-summary-storage](https://github.com/confighub/cub-scout/tree/main/examples/connected-summary-storage)

## What It Reads

It reads:

- the copied summary-store fixture under `fixtures/summary-store/`
- the `cub-scout` binary

## What It Writes

It writes local verification output under `sample-output/`:

- `summary-list-all.json`
- `summary-list-kind-dev-prod.json`
- `summary-slack-dry-run.json`

It does not write ConfigHub state and does not mutate live infrastructure.

## Read-Only First

```bash
cd incubator/connected-summary-storage
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

`./setup.sh` is read-only with respect to ConfigHub and live infrastructure. It only reads the copied summary fixture and writes local output files.

`./verify.sh` is read-only. It checks that the expected records and digest fields are present.

`./cleanup.sh` only removes local sample output.

## What Success Looks Like

At the summary-list level you should see:

- four total records in the unfiltered JSON list
- two records for `kind-dev/prod`
- both `scan` and `gitops-status` record types represented

At the Slack digest level you should see:

- a dry-run payload with `text` and `blocks`
- `kind-dev` in the cluster list
- a next action containing `cub-scout summary list`

## Evidence To Check

```bash
./verify.sh
jq '{count, entries}' sample-output/summary-list-all.json
jq '{count, entries}' sample-output/summary-list-kind-dev-prod.json
jq '{text, blocks}' sample-output/summary-slack-dry-run.json
```

## Why This Example Matters

This gives the incubator set a reporting and automation story built on stored evidence rather than live cluster access.

It answers a practical question quickly:

- once connected summaries exist, how do we query them and turn them into a digest another system could consume?

That makes it a good companion to the import and compare examples, because it shifts from collection into retention, filtering, and follow-on automation.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
