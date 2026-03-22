# AI Start Here

Use this page when you want to drive `import-from-live` safely with an AI assistant.

## What This Example Is For

This example is for brownfield discovery from a running cluster.

It creates a small local `kind` cluster with mixed Argo, Helm, and native ownership signals, then runs `cub-scout import --dry-run --json` to propose a ConfigHub structure.

It does not mutate ConfigHub by default.

## Read-Only First

Start here:

```bash
cd incubator/import-from-live
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
- `cub-scout import --yes` is a separate manual follow-on because it mutates ConfigHub

## What To Verify

```bash
kubectl get application -n argocd
kubectl get deployment -n myapp-dev
kubectl get statefulset -n myapp-prod
jq '.appSpace' sample-output/suggestion.json
jq '.units[] | {slug, app, variant}' sample-output/suggestion.json
```

Use the evidence like this:

- `kubectl` proves the live cluster fixtures exist
- `cub-scout import --dry-run --json` proves the proposed ConfigHub structure
- the committed expected output proves the suggestion contract stayed stable for this example

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
