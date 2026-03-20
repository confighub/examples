# AI Start Here

Use this page when you want to drive `gitops-import-argo` safely with Codex, Claude, Cursor, or another AI assistant.

## What This Example Is For

This example is for GitOps import from a real ArgoCD environment into ConfigHub.

It is the import-and-evidence path, not the layered recipe path.

## Read-Only First

Start with preview commands only:

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh --explain-json | jq
which cub || true
kubectl version --client 2>/dev/null || true
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

Cluster and ArgoCD only:

```bash
./setup.sh
./verify.sh
```

With worker and import path:

```bash
cub auth login
export CUB_SPACE=<space>
./setup.sh --with-worker
./verify.sh
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" worker-kubernetes-yaml-cluster --json | jq
cub gitops import --space "$CUB_SPACE" worker-kubernetes-yaml-cluster worker-argocdrenderer-kubernetes-yaml-cluster --wait
```

With optional contrast fixtures:

```bash
./setup.sh --with-worker --with-contrast
```

If another local example is already using `9080`, this example will choose a free ArgoCD host port in `9080-9099` and record it in `var/argocd-host-port.txt`. You can override that with `export ARGOCD_HOST_PORT=<port>` before running `./setup.sh`.

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` mutates live infrastructure
- `./setup.sh --with-worker` mutates ConfigHub and live infrastructure
- `cub gitops discover` mutates ConfigHub only
- `cub gitops import` mutates ConfigHub only
- `./cleanup.sh` deletes the local kind cluster and local kubeconfig state

## What To Verify

Cluster side:

```bash
export KUBECONFIG=$PWD/var/gitops-import-argo.kubeconfig
cat var/argocd-host-port.txt
kubectl get applications -n argocd
kubectl get all -A
```

ConfigHub side:

```bash
cub unit list --space "$CUB_SPACE" --json | jq
cub unit-action list --space "$CUB_SPACE" <unit-slug>
```

Optional `cub-scout` side:

```bash
cub-scout gitops status
cub-scout map list
```

Use the evidence like this:

- `kubectl` proves raw cluster facts
- ConfigHub proves import and renderer facts
- `cub-scout` proves live ownership and GitOps context facts

Do not collapse those into one claim.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
