# AI Start Here

Use this page when you want to drive `gitops-import-argo` safely with Codex, Claude, Cursor, or another AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint explicitly
5. ask `Ready to continue?`
6. wait for the human before continuing

## Suggested Prompt

```text
Read incubator/gitops-import-argo/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Example Is For

This example is for GitOps import from a real ArgoCD environment into ConfigHub.

It is the import-and-evidence path, not the layered recipe path.

The standard story here is the healthy guestbook path. Do not add `--with-contrast` until after the human has already seen value from that path.

## Stage 1: Preview The Plan (read-only)

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh --explain-json | jq
which cub || true
kubectl version --client 2>/dev/null || true
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- none yet; this stage is CLI-only preview

Pause after this stage.

## Stage 2: Build The Local Argo Environment (mutates live infrastructure)

Cluster and ArgoCD only:

```bash
./setup.sh
```

With optional contrast fixtures:

```bash
./setup.sh --with-contrast
```

If another local example is already using `9080`, this example will choose a free ArgoCD host port in `9080-9099` and record it in `var/argocd-host-port.txt`.

GUI checkpoint:

- Argo CD UI: `https://localhost:$(cat var/argocd-host-port.txt)`
- certificate warning is expected in this local setup

Pause after this stage.

## Stage 3: Verify Cluster And Argo Evidence (read-only)

```bash
export KUBECONFIG=$PWD/var/gitops-import-argo.kubeconfig
./verify.sh
cat var/argocd-host-port.txt
kubectl get applications -n argocd
kubectl get all -A
```

Use the evidence like this:

- `kubectl` proves raw cluster facts
- Argo UI and Argo objects prove controller-side facts

What this does not prove:

- no ConfigHub import has happened yet

Pause after this stage.

## Stage 4: Connect Worker And Discover (mutates ConfigHub)

```bash
cub auth login
export CUB_SPACE=<space>
./setup.sh --with-worker
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" worker-kubernetes-yaml-cluster --json | jq
```

What this mutates:

- ConfigHub space state
- local worker process and local Argo API access

GUI checkpoint:

- ConfigHub GUI: open space `$CUB_SPACE` and inspect targets and discovered units

Pause after this stage.

## Stage 5: Import And Verify ConfigHub Evidence (mutates ConfigHub)

```bash
cub gitops import --space "$CUB_SPACE" worker-kubernetes-yaml-cluster worker-argocdrenderer-kubernetes-yaml-cluster --wait
cub unit list --space "$CUB_SPACE" --json | jq
cub unit-action list --space "$CUB_SPACE" <unit-slug>
```

Optional `cub-scout` side:

```bash
cub-scout gitops status
cub-scout map list
```

Use the evidence like this:

- ConfigHub proves import and renderer facts
- `cub-scout` proves live ownership and GitOps context facts

What this does not prove:

- the contrast path is intentionally mixed; healthy and unhealthy apps should be reported separately

GUI checkpoint:

- ConfigHub GUI: open space `$CUB_SPACE`, review units, links, and recent actions

Pause after this stage.

## Stage 6: Cleanup

```bash
./cleanup.sh
```

This deletes the local kind cluster and local kubeconfig state.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
