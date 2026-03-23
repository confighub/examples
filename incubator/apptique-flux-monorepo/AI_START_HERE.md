# AI Start Here

Use this page when you want to drive `apptique-flux-monorepo` safely with an AI assistant.

## What This Example Is For

This example is for a realistic Flux app layout with base plus overlays.

It creates its own local `kind` cluster, installs the Flux controllers it needs, and uses a dedicated kubeconfig under `var/`.

It is not an import example and it does not mutate ConfigHub by itself.

## Read-Only First

Start here:

```bash
cd incubator/apptique-flux-monorepo
./setup.sh --explain
./setup.sh --explain-json | jq
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

Optional branch validation:

```bash
EXAMPLES_GIT_REVISION=<branch-name> ./setup.sh --with-prod
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` mutates live infrastructure by creating a cluster, installing Flux, and applying the dev path
- `./setup.sh --with-prod` mutates live infrastructure further by applying the prod path
- `./verify.sh` is read-only
- `./cleanup.sh` deletes the local `kind` cluster and dedicated kubeconfig
- this example never writes ConfigHub state

## What To Verify

Cluster side:

```bash
kubectl --kubeconfig var/apptique-flux-monorepo.kubeconfig get gitrepositories,kustomizations -n flux-system
kubectl --kubeconfig var/apptique-flux-monorepo.kubeconfig get deployment,service -n apptique-dev
kubectl --kubeconfig var/apptique-flux-monorepo.kubeconfig get deployment,service -n apptique-prod
flux --kubeconfig var/apptique-flux-monorepo.kubeconfig get sources git -A
flux --kubeconfig var/apptique-flux-monorepo.kubeconfig get kustomizations -A
```

Optional `cub-scout` side:

```bash
cub-scout map list -q "namespace=apptique-*"
cub-scout trace deployment/frontend -n apptique-dev
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
