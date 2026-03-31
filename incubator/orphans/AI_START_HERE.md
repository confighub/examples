# AI Start Here: orphans

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
Read incubator/orphans/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This example demonstrates orphan discovery in a live cluster.

After the demo, the human will understand:
- How `cub-scout map orphans` finds unmanaged resources
- What `Native` ownership means
- Orphan visibility as explicit risk

This example does NOT mutate ConfigHub.

## Prerequisites

- `kind` for local cluster
- `kubectl` in PATH
- `cub-scout` in PATH

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/orphans
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Creates a local kind cluster
- Applies unmanaged fixture resources
- Discovers orphans with `cub-scout`

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of orphan discovery.

GUI feature ask: Orphan discovery preview in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Build The Environment" (mutates live infrastructure)

Run:

```bash
./setup.sh
```

What to explain:

- Creates local kind cluster
- Applies unmanaged fixtures to `legacy-apps` and `temp-testing` namespaces
- Runs `cub-scout map orphans` to discover Native resources

GUI now: No GUI checkpoint — this is local cluster evidence.

GUI gap: No ConfigHub view of orphan resources.

GUI feature ask: Orphan resources dashboard in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
export KUBECONFIG=$PWD/var/orphans.kubeconfig
kubectl get deployment -n legacy-apps
kubectl get deployment -n temp-testing
jq '.[] | select(.owner == "Native") | {namespace, kind, name}' sample-output/orphans.json
jq '{target, summary}' sample-output/debug-nginx.trace.json
```

What to explain:

- `kubectl` proves unmanaged fixture resources exist
- `map orphans --json` proves resources are discoverable as `Native`
- `trace` shows current native-trace behavior (may include misclassification or non-zero exit)

GUI now: No GUI checkpoint — this is local evidence.

GUI gap: No orphan ownership visualization.

GUI feature ask: Native resource viewer with remediation suggestions. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 4: "Cleanup"

Run:

```bash
./cleanup.sh
```

This deletes the local kind cluster and local sample output.

---

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
