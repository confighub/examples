# AI Start Here: custom-ownership-detectors

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
Read incubator/custom-ownership-detectors/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This example demonstrates extending live ownership detection without writing Go code.

After the demo, the human will understand:
- Custom ownership rules in `detectors.yaml`
- How `cub-scout map` uses custom detectors
- Current gap in `explain` and `trace` (tracked in issue #333)

This example does NOT mutate ConfigHub.

## Prerequisites

- `kind` for local cluster
- `kubectl` in PATH
- `cub-scout` in PATH

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/custom-ownership-detectors
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Creates a local kind cluster
- Applies three native Deployments
- Copies custom `detectors.yaml`

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of custom detectors.

GUI feature ask: Custom detector configuration UI. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Build The Environment" (mutates live infrastructure)

Run:

```bash
./setup.sh
```

What to explain:

- Creates local kind cluster with `detectors-demo` namespace
- Applies three Deployments with platform-team labels
- Runs `cub-scout map` with custom detectors

GUI now: No GUI checkpoint — this is local cluster evidence.

GUI gap: No ConfigHub view of custom ownership rules.

GUI feature ask: Custom detector management in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
kubectl get deployment -n detectors-demo
jq '.[] | {name, owner}' sample-output/map.json
jq '.owner' sample-output/payments-api.explain.json
jq '.owner' sample-output/infra-ui.explain.json
jq '.summary.ownerType' sample-output/payments-api.trace.json
```

What to explain:

- `kubectl` proves fixture workloads exist
- `cub-scout map` proves custom owners appear in inventory
- `explain` currently shows parity gap by falling back to built-in partial-trace summary
- `trace` shows same gap by falling back to native summary
- Gap tracked in [cub-scout issue #333](https://github.com/confighub/cub-scout/issues/333)

GUI now: No GUI checkpoint — this is local evidence.

GUI gap: No custom detector results in ConfigHub GUI.

GUI feature ask: Custom ownership results in unit inspector. No issue filed yet.

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
