# AI Start Here

Use this page when you want to drive `gitops-import-flux` safely with Codex, Claude, Cursor, or another AI assistant.

## What This Example Is For

This example is for GitOps import from a real Flux environment into ConfigHub.

It is the import-and-evidence path, not the layered recipe path.

## Read-Only First

Start with preview commands only:

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh --explain-json | jq
which cub || true
kubectl version --client 2>/dev/null || true
flux --version 2>/dev/null || true
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

Cluster and Flux only:

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
cub gitops discover --space "$CUB_SPACE" <kubernetes-target-slug> --json | jq
cub gitops import --space "$CUB_SPACE" <kubernetes-target-slug> <flux-renderer-target-slug> --wait
```

With optional contrast fixtures:

```bash
./setup.sh --with-worker --with-contrast
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` mutates live infrastructure
- `./setup.sh --with-contrast` mutates live infrastructure further
- `./setup.sh --with-worker` mutates ConfigHub and live infrastructure
- `cub gitops discover` mutates ConfigHub only
- `cub gitops import` mutates ConfigHub only
- `./cleanup.sh` deletes the local kind cluster and local discovery worker state

The worker install step preloads the Flux renderer worker image into kind before rollout. If that step still takes time, wait for the renderer target to appear in `cub target list` before concluding the install is stuck.

## What To Verify

Cluster side:

```bash
export KUBECONFIG=$PWD/var/gitops-import-flux.kubeconfig
flux get all -A
kubectl get gitrepositories,kustomizations,helmreleases -A
kubectl get all -A
```

ConfigHub side:

```bash
cub target list --space "$CUB_SPACE" --json | jq
cub unit list --space "$CUB_SPACE" --json | jq
cub unit-action list --space "$CUB_SPACE" <unit-slug>
```

Optional `cub-scout` side:

```bash
cub-scout gitops status
cub-scout map list
cub-scout tree ownership
```

Use the evidence like this:

- `kubectl` and `flux` prove raw cluster facts
- ConfigHub proves discover and renderer facts
- `cub-scout` proves live ownership and GitOps context facts

Expect a mixed result in the contrast path:

- `podinfo` should be the healthy reference path
- the `platform-config` Kustomizations should be visibly broken because their Git source is not ready
- the two HelmRelease paths should stay source-blocked until their chart sources exist

Do not collapse those into one claim.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
