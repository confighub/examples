# AI Start Here

Use this page when you want to drive `connected-summary-storage` safely with an AI assistant.

## What This Example Is For

This example is for querying stored connected summaries and building a dry-run Slack digest from them.

It reads a copied summary-store fixture and never talks to a live cluster.

It does not mutate ConfigHub.

## Read-Only First

Start here:

```bash
cd incubator/connected-summary-storage
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

```bash
./setup.sh
./verify.sh
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` is read-only with respect to ConfigHub and live infrastructure
- `./verify.sh` is read-only
- `./cleanup.sh` only removes local sample output
- this example never writes ConfigHub state

## What To Verify

```bash
jq '{count, entries}' sample-output/summary-list-all.json
jq '{count, entries}' sample-output/summary-list-kind-dev-prod.json
jq '{text, blocks}' sample-output/summary-slack-dry-run.json
```

Use the evidence like this:

- `summary list --json` proves the stored records are discoverable by time window and filters
- the filtered JSON proves cluster and namespace narrowing works
- `summary slack --dry-run` proves the same stored records can be turned into a machine-readable digest payload without posting to a webhook

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
