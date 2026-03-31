# AI Start Here: flux-boutique

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
Read incubator/flux-boutique/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This is a live Flux fan-out pattern: one GitRepository, many Kustomizations, many services.

After the demo, the human will understand:
- Flux ownership attribution for microservices
- `cub-scout trace` through Flux ownership chain
- Fan-out pattern with shared Git source

This example does NOT mutate ConfigHub.

## Prerequisites

- `kind` for local cluster
- `kubectl` in PATH
- `flux` CLI (optional)
- `cub-scout` in PATH

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/flux-boutique
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Creates a local kind cluster
- Installs Flux
- Applies boutique fixtures

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of Flux fan-out topology.

GUI feature ask: Flux source fan-out visualizer. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Build The Environment" (mutates live infrastructure)

Run:

```bash
./setup.sh
```

What to explain:

- Creates local kind cluster
- Installs Flux
- Applies boutique microservices via Kustomization
- Captures ownership evidence

GUI now: No GUI checkpoint — this is local cluster evidence.

GUI gap: No ConfigHub view of Flux topology.

GUI feature ask: Flux topology import in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
export KUBECONFIG=$PWD/var/flux-boutique.kubeconfig
kubectl get deploy -n boutique
jq '.[] | select(.namespace == "boutique") | {kind, name, owner}' sample-output/map-list.json
jq '{target, summary}' sample-output/trace-payment.json
```

What to explain:

- `kubectl` proves microservice workloads exist
- `map list --json` proves services are seen as Flux-managed
- `trace` proves representative service traces back through Flux ownership
- Note: Deployments may briefly be `NotReady` while images pull; ownership attribution is already correct

GUI now: No GUI checkpoint — this is local evidence.

GUI gap: No Flux ownership chain visualization.

GUI feature ask: Flux provenance chain viewer in ConfigHub. No issue filed yet.

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
