# AI Start Here

Use this page when you want to drive `orphans` safely with an AI assistant.

## What This Example Is For

This example is for orphan discovery in a live cluster.

It creates a local `kind` cluster, applies a copied unmanaged fixture set, and inventories `Native` resources with `cub-scout map orphans --json`.

It does not mutate ConfigHub.

## Read-Only First

Start here:

```bash
cd incubator/orphans
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
- `./setup.sh` mutates live infrastructure and writes local sample output
- `./verify.sh` is read-only with respect to ConfigHub and live infrastructure
- `./cleanup.sh` deletes the local `kind` cluster and local sample output
- this example never writes ConfigHub state

## What To Verify

```bash
kubectl get deployment -n legacy-apps
kubectl get deployment -n temp-testing
jq '.[] | select(.owner == "Native") | {namespace, kind, name}' sample-output/orphans.json
jq '{target, summary}' sample-output/debug-nginx.trace.json
```

Use the evidence like this:

- `kubectl` proves the unmanaged fixture resources exist
- `map orphans --json` proves the same resources are discoverable as `Native`
- the captured `trace` result shows current native-trace behavior for one representative orphan, including any current misclassification or non-zero exit behavior

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
