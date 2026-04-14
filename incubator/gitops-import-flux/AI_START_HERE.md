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

The standard story here is the healthy `podinfo` path. Do not add `--with-contrast` until after the human has already seen value from that path.

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/gitops-import-flux
./setup.sh --explain
./setup.sh --explain-json | jq
which cub || true
kubectl version --client 2>/dev/null || true
flux --version 2>/dev/null || true
```

What to explain:

- Shows what the setup will create
- Checks for required tools

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of planned resources.

GUI feature ask: Setup preview for import examples. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 2: "Build The Local Flux Environment" (mutates live infrastructure)

Ask: "This will create a local kind cluster and install Flux. Ready to proceed?"

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
- Installs Flux
- Applies podinfo and related resources

GUI now: No GUI checkpoint — use Flux CLI and cluster state as source of truth.

GUI gap: No ConfigHub integration visible yet.

GUI feature ask: Flux discovery status in ConfigHub before import. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 3: "Verify Cluster And Flux Evidence" (read-only)

Run:

```bash
export KUBECONFIG=$PWD/var/gitops-import-flux.kubeconfig
./verify.sh
flux get all -A
kubectl get gitrepositories,kustomizations,helmreleases -A
kubectl get all -A
```

What to explain:

- `kubectl` and `flux` prove raw cluster facts
- Contrast path is intentionally mixed — show honestly
- No ConfigHub import has happened yet

GUI now: No GUI checkpoint — this is Flux CLI and kubectl evidence.

GUI gap: No ConfigHub view of these resources yet.

GUI feature ask: Pre-import discovery view in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 4: "Confirm ConfigHub Auth" (read-only gate)

Run:

```bash
cub info
```

What to explain:

- This is a read-only auth gate before any ConfigHub mutation
- If auth is expired, stop here
- The next stage should not start until auth is valid

If auth is expired:

- tell the human to run `cub auth login`
- do not keep retrying in the background
- rerun only the blocked stage after auth is fixed

GUI now: No GUI checkpoint — this is a preflight gate.

GUI gap: No simple session status view for AI/operator auth readiness.

GUI feature ask: lightweight "ready to mutate" preflight page. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 5: "Install Worker And Discover" (mutates ConfigHub)

Ask: "This will create a ConfigHub worker and discover resources. Ready to proceed?"

Run:

```bash
export CUB_SPACE=<space>
./bin/install-worker
cub target list --space "$CUB_SPACE" --json | jq
cub gitops discover --space "$CUB_SPACE" <kubernetes-target-slug> --json | jq
```

What to explain:

- Creates ConfigHub workers
- Expect three targets: Kubernetes, fluxrenderer, fluxoci
- Discovers Flux resources and Kubernetes resources
- If worker install fails due to auth, do not rerun cluster setup; rerun only this stage after auth is fixed

GUI now: Open ConfigHub space and inspect targets and discovered units.

GUI gap: No unified view connecting Flux CLI to ConfigHub discovery.

GUI feature ask: Side-by-side Flux and ConfigHub comparison view. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 6: "Import And Verify ConfigHub Evidence" (mutates ConfigHub)

Ask: "This will import discovered resources into ConfigHub. Ready to proceed?"

Run:

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

What to explain:

- ConfigHub proves discover and renderer facts
- `cub-scout` proves live ownership and GitOps context

GUI now: Open ConfigHub space and review targets, units, and actions.

GUI gap: No visual diff between Flux state and ConfigHub imported state.

GUI feature ask: Import diff view showing before/after comparison. No issue filed yet.

**PAUSE.** Wait for the human.

## Stage 7: "Cleanup"

Run:

```bash
./cleanup.sh
```

This deletes the local kind cluster and local discovery worker state.

---

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
