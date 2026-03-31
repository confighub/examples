# AI Start Here: apptique-argo-app-of-apps

## CRITICAL: Demo Pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen
3. Explain what the output means in plain English
4. STOP and ask "Ready to continue?"
5. Only proceed when the human says to continue

## Suggested Prompt

```text
Read incubator/apptique-argo-app-of-apps/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This is a realistic Argo app-of-apps layout: one root Application with child Applications per environment.

This example does NOT mutate ConfigHub.

## Prerequisites

- `kind` for local cluster
- `kubectl` in PATH
- Argo CD CLI (optional)

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/apptique-argo-app-of-apps
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Creates a local kind cluster
- Installs Argo CD
- Applies root Application

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of Argo app-of-apps structure.

GUI feature ask: App-of-apps topology viewer. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Build The Environment" (mutates live infrastructure)

Run:

```bash
./setup.sh
```

Optional branch validation:

```bash
EXAMPLES_GIT_REVISION=<branch-name> ./setup.sh
```

What to explain:

- Creates local kind cluster
- Installs Argo CD
- Applies root Application which creates child Applications

GUI now: Argo CD UI (if port-forwarded) shows the app-of-apps hierarchy.

GUI gap: No ConfigHub view of this structure yet.

GUI feature ask: Argo topology import in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
kubectl --kubeconfig var/apptique-argo-app-of-apps.kubeconfig get applications -n argocd
kubectl --kubeconfig var/apptique-argo-app-of-apps.kubeconfig get deployment,service -n apptique-dev
kubectl --kubeconfig var/apptique-argo-app-of-apps.kubeconfig get deployment,service -n apptique-prod
```

Optional `cub-scout` side:

```bash
cub-scout map list -q "owner=ArgoCD"
cub-scout trace deployment/frontend -n apptique-dev
```

What to explain:

- `kubectl` proves raw Argo and workload facts
- `cub-scout` proves ownership and provenance

GUI now: No GUI checkpoint — this is local Argo evidence.

GUI gap: No ConfigHub integration for local Argo clusters.

GUI feature ask: Local cluster discovery workflow. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 4: "Cleanup"

Run:

```bash
./cleanup.sh
```

This deletes the local kind cluster and dedicated kubeconfig.

---

## Follow-On

To bring an Argo-managed cluster like this into ConfigHub:

- [../gitops-import-argo](../gitops-import-argo/README.md)

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
