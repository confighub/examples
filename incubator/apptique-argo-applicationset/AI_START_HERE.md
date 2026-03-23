# AI Start Here

Use this page when you want to drive `apptique-argo-applicationset` safely with an AI assistant.

## What This Example Is For

This example is for a realistic Argo ApplicationSet app layout.

It creates its own local `kind` cluster, installs Argo CD, and uses a dedicated kubeconfig under `var/`.

It is not an import example and it does not mutate ConfigHub by itself.

## Read-Only First

Start here:

```bash
cd incubator/apptique-argo-applicationset
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

```bash
./setup.sh
./verify.sh
```

Optional branch validation:

```bash
EXAMPLES_GIT_REVISION=<branch-name> ./setup.sh
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` mutates live infrastructure by creating a cluster, installing Argo CD, and applying the ApplicationSet
- `./verify.sh` is read-only
- `./cleanup.sh` deletes the local `kind` cluster and dedicated kubeconfig
- this example never writes ConfigHub state

## What To Verify

Cluster side:

```bash
kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get applicationsets,applications -n argocd
kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get deployment,service -n apptique-dev
kubectl --kubeconfig var/apptique-argo-applicationset.kubeconfig get deployment,service -n apptique-prod
```

Optional `cub-scout` side:

```bash
cub-scout map list -q "owner=ArgoCD"
cub-scout trace deployment/frontend -n apptique-dev
```

Use the evidence like this:

- `kubectl` proves raw Argo and workload facts
- `cub-scout` proves ownership and provenance

## Follow-On

If the human wants to bring an Argo-managed cluster like this into ConfigHub, continue with:

- [../gitops-import-argo](../gitops-import-argo/README.md)

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
