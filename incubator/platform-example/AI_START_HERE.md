# AI Start Here

Use this page when you want to drive `platform-example` safely with an AI assistant.

## What This Example Is For

This example is for a realistic mixed cluster:

- Flux-managed platform resources
- unmanaged orphan resources next to them
- one representative Flux trace

It creates a local `kind` cluster, installs Flux, applies a copied podinfo GitRepository and Kustomization, applies copied orphan fixtures, and captures ownership evidence with `cub-scout`.

It does not mutate ConfigHub.

## Read-Only First

Start here:

```bash
cd incubator/platform-example
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
- `./cleanup.sh` deletes the local `kind` cluster, local kubeconfig, and local sample output
- this example never writes ConfigHub state

## What To Verify

```bash
cat sample-output/flux-status.txt
jq '.[] | select(.owner == "Flux") | {namespace, kind, name}' sample-output/map-list.json
jq '.[] | select(.owner == "Native") | {namespace, kind, name}' sample-output/orphans.json
jq '{target, summary}' sample-output/trace-podinfo.json
```

Use the evidence like this:

- `flux get all -A` proves the GitOps controllers and source objects are present
- `map list --json` proves Flux-managed and Native resources coexist in the same cluster
- `map orphans --json` proves the unmanaged fixtures are still visible as explicit risk
- `trace` proves a representative GitOps workload can still be traced back to source

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
