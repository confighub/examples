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

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/gitops-import-argo
./setup.sh --explain
./setup.sh --explain-json | jq
which cub || true
kubectl version --client 2>/dev/null || true
```

What to explain:

- Shows what the setup will create
- Checks for required tools

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of planned resources.

GUI feature ask: Setup preview for import examples. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 2: "Build The Local Argo Environment" (mutates live infrastructure)

Ask: "This will create a local kind cluster and install Argo CD. Ready to proceed?"

Run:

```bash
./setup.sh
```

With optional contrast fixtures:

```bash
./setup.sh --with-contrast
```

What to explain:

- Creates a local kind cluster
- Installs Argo CD
- Applies guestbook applications
- If port 9080 is in use, chooses a free port in 9080-9099

GUI now: Argo CD UI at `https://localhost:$(cat var/argocd-host-port.txt)`. Certificate warning is expected.

GUI gap: No ConfigHub integration visible yet — this is local Argo only.

GUI feature ask: Argo discovery status in ConfigHub before import. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 3: "Verify Cluster And Argo Evidence" (read-only)

Run:

```bash
export KUBECONFIG=$PWD/var/gitops-import-argo.kubeconfig
./verify.sh
cat var/argocd-host-port.txt
kubectl get applications -n argocd
kubectl get all -A
```

What to explain:

- `kubectl` proves raw cluster facts
- Argo UI and Argo objects prove controller-side facts
- No ConfigHub import has happened yet

GUI now: Argo CD UI shows Applications synced and healthy.

GUI gap: No ConfigHub view of these applications yet.

GUI feature ask: Pre-import discovery view in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 4: "Connect Worker And Discover" (mutates ConfigHub)

Ask: "This will create a ConfigHub worker and discover resources. Ready to proceed?"

Run:

```bash
cub auth login
export CUB_SPACE=<space>
./setup.sh --with-worker
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" worker-kubernetes-yaml-cluster --json | jq
```

What to explain:

- Creates a ConfigHub worker
- Discovers Argo applications and Kubernetes resources
- Results visible in ConfigHub

GUI now: Open ConfigHub space and inspect targets and discovered units.

GUI gap: No unified view connecting Argo UI to ConfigHub discovery.

GUI feature ask: Side-by-side Argo and ConfigHub comparison view. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 5: "Import And Verify ConfigHub Evidence" (mutates ConfigHub)

Ask: "This will import discovered resources into ConfigHub. Ready to proceed?"

Run:

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

What to explain:

- ConfigHub proves import and renderer facts
- `cub-scout` proves live ownership and GitOps context
- Contrast path is intentionally mixed — healthy and unhealthy apps reported separately

GUI now: Open ConfigHub space and review units, links, and recent actions.

GUI gap: No visual diff between Argo state and ConfigHub imported state.

GUI feature ask: Import diff view showing before/after comparison. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 6: "Cleanup"

Run:

```bash
./cleanup.sh
```

This deletes the local kind cluster and local kubeconfig state.

---

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
