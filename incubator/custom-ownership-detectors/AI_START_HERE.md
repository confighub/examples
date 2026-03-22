# AI Start Here

Use this page when you want to drive `custom-ownership-detectors` safely with an AI assistant.

## What This Example Is For

This example is for extending live ownership detection without writing Go code.

It creates a small local `kind` cluster, applies three native Deployments, points `cub-scout` at a copied `detectors.yaml`, verifies custom owners in `map`, and captures the current `explain` and `trace` gap explicitly.

It does not mutate ConfigHub.

## Read-Only First

Start here:

```bash
cd incubator/custom-ownership-detectors
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
kubectl get deployment -n detectors-demo
jq '.[] | {name, owner}' sample-output/map.json
jq '.owner' sample-output/payments-api.explain.json
jq '.owner' sample-output/infra-ui.explain.json
jq '.summary.ownerType' sample-output/payments-api.trace.json
```

Use the evidence like this:

- `kubectl` proves the fixture workloads exist
- `cub-scout map` proves custom owners appear in inventory
- `cub-scout explain` currently shows the parity gap by falling back to the built-in partial-trace summary
- `cub-scout trace` currently shows the same gap by falling back to a native summary
- the gap is tracked upstream in [cub-scout issue #333](https://github.com/confighub/cub-scout/issues/333)

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
