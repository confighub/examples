# AI Start Here

Use this page when you want to drive `flux-boutique` safely with an AI assistant.

## What This Example Is For

This example is for a live Flux fan-out pattern: one GitRepository, many Kustomizations, many services.

It creates a local `kind` cluster, installs Flux, applies a copied fixture, and captures ownership evidence with `cub-scout`.

It does not mutate ConfigHub.

## Read-Only First

Start here:

```bash
cd incubator/flux-boutique
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
kubectl get deploy -n boutique
jq '.[] | select(.namespace == "boutique") | {kind, name, owner}' sample-output/map-list.json
jq '{target, summary}' sample-output/trace-payment.json
```

Use the evidence like this:

- `kubectl` proves the microservice workloads exist
- `map list --json` proves the services are seen as Flux-managed
- `trace` proves a representative service can be traced back through Flux ownership

Do not overclaim workload health from the first capture alone. The boutique Deployments can still be `NotReady` briefly while images are pulling, even when Flux reconciliation and ownership attribution are already correct.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
