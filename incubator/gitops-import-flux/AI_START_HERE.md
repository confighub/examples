# AI Start Here

Use this page when you want to drive `gitops-import-flux` safely with Codex, Claude, Cursor, or another AI assistant.

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
Read incubator/gitops-import-flux/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Example Is For

This example is for GitOps import from a real Flux environment into ConfigHub.

It is the import-and-evidence path, not the layered recipe path.

## Stage 1: Preview The Plan (read-only)

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh --explain-json | jq
which cub || true
kubectl version --client 2>/dev/null || true
flux --version 2>/dev/null || true
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- none yet; this stage is CLI-only preview

Pause after this stage.

## Stage 2: Build The Local Flux Environment (mutates live infrastructure)

Cluster and Flux only:

```bash
./setup.sh
```

With optional contrast fixtures:

```bash
./setup.sh --with-contrast
```

GUI checkpoint:

- none by default; use the Flux CLI and cluster state as the source of truth

Pause after this stage.

## Stage 3: Verify Cluster And Flux Evidence (read-only)

```bash
export KUBECONFIG=$PWD/var/gitops-import-flux.kubeconfig
./verify.sh
flux get all -A
kubectl get gitrepositories,kustomizations,helmreleases -A
kubectl get all -A
```

Use the evidence like this:

- `kubectl` and `flux` prove raw cluster facts
- the contrast path is intentionally mixed and should be shown honestly

What this does not prove:

- no ConfigHub import has happened yet

Pause after this stage.

## Stage 4: Connect Worker And Discover (mutates ConfigHub)

```bash
cub auth login
export CUB_SPACE=<space>
./setup.sh --with-worker
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" <kubernetes-target-slug> --json | jq
```

If the worker install succeeds, expect three useful targets:

- one Kubernetes discovery target
- one `fluxrenderer` target for import and render
- one `fluxoci` target for Flux-managed deployment of raw Kubernetes manifests

GUI checkpoint:

- ConfigHub GUI: open space `$CUB_SPACE` and inspect targets plus discovered units

Pause after this stage.

## Stage 5: Import And Verify ConfigHub Evidence (mutates ConfigHub)

```bash
cub gitops import --space "$CUB_SPACE" <kubernetes-target-slug> <flux-renderer-target-slug> --wait
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

- ConfigHub proves discover and renderer facts
- `cub-scout` proves live ownership and GitOps context facts

GUI checkpoint:

- ConfigHub GUI: open space `$CUB_SPACE`, review targets, units, and actions

Pause after this stage.

## Stage 6: Cleanup

```bash
./cleanup.sh
```

This deletes the local kind cluster and local discovery worker state.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
