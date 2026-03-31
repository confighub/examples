# AI Start Here: platform-example

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
Read incubator/platform-example/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This is a realistic mixed cluster example:
- Flux-managed platform resources
- Unmanaged orphan resources next to them
- Representative Flux trace

After the demo, the human will understand:
- How managed and unmanaged resources coexist
- How `cub-scout` distinguishes ownership
- Trace through Flux ownership chain

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
cd incubator/platform-example
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Creates a local kind cluster
- Installs Flux
- Applies podinfo and orphan fixtures

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of mixed ownership topology.

GUI feature ask: Mixed cluster topology preview. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Build The Environment" (mutates live infrastructure)

Run:

```bash
./setup.sh
```

What to explain:

- Creates local kind cluster
- Installs Flux with GitRepository and Kustomization
- Applies orphan fixtures alongside managed resources
- Captures ownership evidence

GUI now: No GUI checkpoint — this is local cluster evidence.

GUI gap: No ConfigHub view of mixed ownership.

GUI feature ask: Mixed ownership dashboard in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
export KUBECONFIG=$PWD/var/platform-example.kubeconfig
cat sample-output/flux-status.txt
jq '.[] | select(.owner == "Flux") | {namespace, kind, name}' sample-output/map-list.json
jq '.[] | select(.owner == "Native") | {namespace, kind, name}' sample-output/orphans.json
jq '{target, summary}' sample-output/trace-podinfo.json
```

What to explain:

- `flux get all -A` proves GitOps controllers and sources are present
- `map list --json` proves Flux-managed and Native resources coexist
- `map orphans --json` proves unmanaged fixtures are visible as explicit risk
- `trace` proves GitOps workload can be traced back to source

GUI now: No GUI checkpoint — this is local evidence.

GUI gap: No unified managed/unmanaged view.

GUI feature ask: Ownership contrast view in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 4: "Cleanup"

Run:

```bash
./cleanup.sh
```

This deletes the local kind cluster, local kubeconfig, and local sample output.

---

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
