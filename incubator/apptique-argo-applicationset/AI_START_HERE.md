# AI Start Here

Use this page when you want to drive the incubator `apptique-argo-applicationset` example safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. say what the GUI shows today
6. say what the GUI does not show yet
7. name the GUI feature ask and cite the issue number if one exists; if not, say that explicitly
8. tell the human to open the GUI and give them time to inspect it
9. ask `Ready to continue?`
10. do not move on until the human says to continue

## Suggested Prompt

```text
Read incubator/apptique-argo-applicationset/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This example is for a realistic Argo ApplicationSet app layout.

It creates its own local `kind` cluster, installs Argo CD, and uses a dedicated kubeconfig under `var/`.

It is not an import example and it does not mutate ConfigHub by itself.

## Stage 1: Preview The Plan (read-only)

```bash
cd incubator/apptique-argo-applicationset
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI surface for the ApplicationSet plan before the local cluster exists
- GUI feature ask: no issue filed yet for a plan-oriented GUI handoff on this example

Pause after this stage.

## Stage 2: Create The ApplicationSet Path (mutates live infrastructure)

```bash
./setup.sh
```

Optional branch validation:

```bash
EXAMPLES_GIT_REVISION=<branch-name> ./setup.sh
```

Important boundaries:

- `./setup.sh` mutates live infrastructure by creating a cluster, installing Argo CD, and applying the ApplicationSet
- it does not mutate ConfigHub

What you should see after:

- `ApplicationSet/apptique` in `argocd`
- generated `Application/apptique-dev` and `Application/apptique-prod`
- `Deployment/frontend` and `Service/frontend` in both namespaces

GUI checkpoint:

- GUI now: if you port-forward Argo CD or already expose it, you can inspect the generated Applications there
- GUI gap: this example does not automatically hand you a ready Argo CD GUI URL or a focused ApplicationSet landing view
- GUI feature ask: no issue filed yet for a tighter ApplicationSet-focused GUI handoff in this example

Pause after this stage.

## Stage 3: Verify The Evidence (read-only)

```bash
./verify.sh
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

GUI checkpoint:

- GUI now: the Argo UI can show the generated Applications if you have it open separately
- GUI gap: there is no built-in side-by-side CLI-to-GUI handoff in this example
- GUI feature ask: no issue filed yet for an evidence view that pairs generated Applications with the resulting workloads

Pause after this stage.

## Stage 4: Follow-On And Cleanup

If the human wants to bring an Argo-managed cluster like this into ConfigHub, continue with:

- [../incubator/gitops-import-argo](../incubator/gitops-import-argo/README.md)

Cleanup:

```bash
./cleanup.sh
```

This deletes the local `kind` cluster and dedicated kubeconfig.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
