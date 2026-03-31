# AI Start Here: combined-git-live

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
Read incubator/combined-git-live/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This example demonstrates Git intent versus live cluster alignment.

This example does NOT mutate ConfigHub.

## Prerequisites

- Reachable Kubernetes cluster
- `kubectl` in PATH
- `cub-scout` in PATH

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/combined-git-live
./setup.sh --explain
./setup.sh --explain-json | jq
kubectl config current-context
```

What to explain:

- Shows what fixtures will be applied
- Shows current cluster context

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of Git vs live alignment.

GUI feature ask: Drift preview before applying fixtures. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Build The Environment" (mutates live infrastructure)

Run:

```bash
./setup.sh
```

What to explain:

- Applies fixture resources to payment-dev and payment-prod namespaces
- Runs `cub-scout combined` to capture alignment

GUI now: No GUI checkpoint — this is local cluster evidence.

GUI gap: No ConfigHub integration for combined alignment.

GUI feature ask: Git vs live alignment view in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
kubectl get deployment -n payment-dev
kubectl get deployment -n payment-prod
jq '.alignment' sample-output/alignment.json
jq '.alignment[] | select(.status != "aligned")' sample-output/alignment.json
```

What to explain:

- `kubectl` proves the observed cluster state
- `cub-scout combined` proves how Git and live state line up
- Non-aligned entries show drift or gaps

GUI now: No GUI checkpoint — this is local evidence.

GUI gap: No visual drift detection dashboard.

GUI feature ask: Drift detection dashboard in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 4: "Cleanup"

Run:

```bash
./cleanup.sh
```

This deletes the fixture namespaces.

---

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
