# AI Start Here

Use this page when you want to drive `apptique-flux-monorepo` safely with an AI assistant.

## What This Example Is For

This example is for a realistic Flux app layout with base plus overlays.

It is not an import example and it does not mutate ConfigHub by itself.

## Read-Only First

Start here:

```bash
cd incubator/apptique-flux-monorepo
./setup.sh --explain
./setup.sh --explain-json | jq
kubectl config current-context
flux get all -A
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

Dev only:

```bash
./setup.sh
./verify.sh
```

Dev and prod:

```bash
./setup.sh --with-prod
./verify.sh --with-prod
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` mutates live infrastructure
- `./setup.sh --with-prod` mutates live infrastructure further
- `./verify.sh` is read-only
- `./cleanup.sh` deletes the live example resources

This example assumes Flux is already installed in the current cluster.

## What To Verify

Cluster side:

```bash
kubectl get gitrepositories,kustomizations -n flux-system
kubectl get deployment,service -n apptique-dev
kubectl get deployment,service -n apptique-prod
flux get sources git -A
flux get kustomizations -A
```

Optional `cub-scout` side:

```bash
cub-scout map list -q "namespace=apptique-*"
cub-scout trace deployment/frontend -n apptique-dev
cub-scout gitops status
```

Use the evidence like this:

- `kubectl` and `flux` prove raw GitOps and workload facts
- `cub-scout` proves ownership and provenance

## Follow-On

If the human wants to bring a Flux-managed cluster like this into ConfigHub, continue with:

- [../gitops-import-flux](../gitops-import-flux/README.md)

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
