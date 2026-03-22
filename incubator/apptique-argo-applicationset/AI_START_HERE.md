# AI Start Here

Use this page when you want to drive `apptique-argo-applicationset` safely with an AI assistant.

## What This Example Is For

This example is for a realistic Argo ApplicationSet app layout.

It is not an import example and it does not mutate ConfigHub by itself.

## Read-Only First

Start here:

```bash
cd incubator/apptique-argo-applicationset
./setup.sh --explain
./setup.sh --explain-json | jq
kubectl config current-context
kubectl get applicationsets -n argocd 2>/dev/null || true
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
- `./verify.sh` is read-only
- `./cleanup.sh` deletes the live example resources

This example assumes Argo CD is already installed in the current cluster.

## What To Verify

Cluster side:

```bash
kubectl get applicationsets,applications -n argocd
kubectl get deployment,service -n apptique-dev
kubectl get deployment,service -n apptique-prod
```

Optional `cub-scout` side:

```bash
cub-scout map list -q "owner=ArgoCD"
cub-scout trace deployment/frontend -n apptique-dev
cub-scout gitops status
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
