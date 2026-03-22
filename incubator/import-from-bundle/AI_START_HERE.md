# AI Start Here

Use this page when you want to drive `import-from-bundle` safely with an AI assistant.

## What This Example Is For

This is an offline import proposal example.

It is useful when the human wants dry-run import evidence without cluster access.

## Read-Only First

Start here:

```bash
cd incubator/import-from-bundle
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
jq '.evidence' sample-output/suggestion.json
jq '.proposal' sample-output/suggestion.json
jq '.workloads' sample-output/suggestion.json
```

Use the evidence like this:

- `evidence` proves the bundle source path story
- `proposal` proves the dry-run import model
- `workloads` proves what the bundle contributed

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
