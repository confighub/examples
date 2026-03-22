# AI Start Here

Use this page when you want to drive `lifecycle-hazards` safely with an AI assistant.

## What This Example Is For

This example is for detecting Helm-to-Argo migration hazards from a manifest file.

It inventories hooks with `cub-scout map hooks --file` and scans for lifecycle hazards with `cub-scout scan --file --lifecycle-hazards --json`.

It does not mutate ConfigHub.
It does not mutate live infrastructure.

## Read-Only First

Start here:

```bash
cd incubator/lifecycle-hazards
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
- `./setup.sh` writes local JSON output only
- `./verify.sh` is read-only with respect to ConfigHub and live infrastructure
- `./cleanup.sh` removes local sample output only
- this example never writes ConfigHub state

## What To Verify

```bash
jq '.hooks[] | {name, mappedPhase}' sample-output/hooks.json
jq '.lifecycleHazards.summary' sample-output/lifecycle-scan.json
jq '.lifecycleHazards.findings[] | {rule, resource}' sample-output/lifecycle-scan.json
jq '.static.findings[] | {resource_name, severity}' sample-output/lifecycle-scan.json
```

Use the evidence like this:

- `map hooks` proves how Helm and Argo hooks are interpreted
- `scan --lifecycle-hazards` proves which hook patterns become risky under Argo CD
- the static findings remind us that lifecycle hazards are only one slice of file-based risk detection

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
