# AI Start Here

Use this page when you want to drive `apptique-flux-monorepo` safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. ask `Ready to continue?`
6. do not move on until the human says to continue

## Suggested Prompt

```text
Read incubator/apptique-flux-monorepo/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Example Is For

This example is for a realistic Flux app layout with base plus overlays.

It creates its own local `kind` cluster, installs the Flux controllers it needs, and uses a dedicated kubeconfig under `var/`.

It is not an import example and it does not mutate ConfigHub by itself.

## Stage 1: Preview The Plan (read-only)

```bash
cd incubator/apptique-flux-monorepo
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- none yet; this stage is CLI-only preview

Pause after this stage.

## Stage 2: Create The Dev Path (mutates live infrastructure)

```bash
./setup.sh
```

What this mutates:

- local `kind` cluster
- Flux installation on that cluster
- dev namespace and workloads

What you should see after:

- Flux controllers in `flux-system`
- one GitRepository and one Kustomization path for dev
- `apptique-dev` Deployment and Service

GUI checkpoint:

- Flux has no separate built-in GUI in this example; use the CLI evidence below as the canonical view

Pause after this stage.

## Stage 3: Verify Dev Evidence (read-only)

```bash
./verify.sh
kubectl --kubeconfig var/apptique-flux-monorepo.kubeconfig get gitrepositories,kustomizations -n flux-system
kubectl --kubeconfig var/apptique-flux-monorepo.kubeconfig get deployment,service -n apptique-dev
flux --kubeconfig var/apptique-flux-monorepo.kubeconfig get sources git -A
flux --kubeconfig var/apptique-flux-monorepo.kubeconfig get kustomizations -A
```

What this proves:

- `kubectl` and `flux` prove raw GitOps and workload facts for dev

What this does not prove:

- no ConfigHub mutation
- prod is not included unless you choose the next stage

Optional ownership checkpoint:

```bash
cub-scout map list -q "namespace=apptique-*"
cub-scout trace deployment/frontend -n apptique-dev
```

Pause after this stage.

## Stage 4: Add Prod (mutates live infrastructure)

```bash
./setup.sh --with-prod
./verify.sh --with-prod
```

Optional branch-backed validation:

```bash
EXAMPLES_GIT_REVISION=<branch-name> ./setup.sh --with-prod
```

What you should see after:

- `apptique-prod` Deployment and Service
- both dev and prod Kustomizations ready

GUI checkpoint:

- none; use Flux CLI and `kubectl` as the source of truth

Pause after this stage.

## Stage 5: Follow-On And Cleanup

If the human wants to bring a Flux-managed cluster like this into ConfigHub, continue with:

- [../gitops-import-flux](../gitops-import-flux/README.md)

Cleanup:

```bash
./cleanup.sh
```

This deletes the local `kind` cluster and dedicated kubeconfig.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
