# AI Start Here

Use this page when you want to drive `combined-git-live` safely with an AI assistant.

## What This Example Is For

This example is for Git intent versus live cluster alignment.

It does not mutate ConfigHub by itself.

## Read-Only First

Start here:

```bash
cd incubator/combined-git-live
./setup.sh --explain
./setup.sh --explain-json | jq
kubectl config current-context
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

```bash
./setup.sh
./verify.sh
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` mutates live infrastructure
- `./verify.sh` is read-only with respect to ConfigHub and live infrastructure
- `./cleanup.sh` deletes the fixture namespaces

This example assumes a reachable Kubernetes cluster and `cub-scout`.

## What To Verify

```bash
kubectl get deployment -n payment-dev
kubectl get deployment -n payment-prod
jq '.alignment' sample-output/alignment.json
jq '.alignment[] | select(.status != "aligned")' sample-output/alignment.json
```

Use the evidence like this:

- `kubectl` proves the observed cluster state exists
- `cub-scout combined` proves how Git and live state line up

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
