# AI Start Here

Use this page when you want to drive `fleet-import` safely with an AI assistant.

## What This Example Is For

This is a multi-cluster aggregation example.

It is useful when the human wants to merge existing per-cluster import results into one fleet proposal.

## Read-Only First

Start here:

```bash
cd incubator/fleet-import
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
- `./setup.sh` writes local output only
- `./verify.sh` is read-only with respect to ConfigHub and live infrastructure
- `./cleanup.sh` removes local sample output only

This example does not require a live cluster.
It does require `cub-scout`.

## What To Verify

```bash
jq '.summary' sample-output/fleet-summary.json
jq '.proposal' sample-output/fleet-summary.json
jq '.summary.byApp | to_entries[] | select(.value | length > 1)' sample-output/fleet-summary.json
```

Use the evidence like this:

- `summary` proves the fleet rollup
- `proposal` proves the unified app model suggestion
- `byApp` proves which apps span multiple clusters

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
