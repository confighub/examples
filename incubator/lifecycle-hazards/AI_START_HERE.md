# AI Start Here: lifecycle-hazards

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
Read incubator/lifecycle-hazards/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This example detects Helm-to-Argo migration hazards from a manifest file.

After the demo, the human will understand:
- How `cub-scout map hooks` interprets Helm and Argo hooks
- Which hook patterns become risky under Argo CD
- Lifecycle hazard scanning with `cub-scout scan`

This example does NOT mutate ConfigHub.
This example does NOT mutate live infrastructure.

## Prerequisites

- `cub-scout` in PATH
- `jq` for JSON inspection

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/lifecycle-hazards
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Scans fixture manifest files for hooks
- Detects lifecycle hazards
- No cluster needed

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of hook analysis.

GUI feature ask: Hook analysis preview in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Run The Scan" (local files only)

Run:

```bash
./setup.sh
```

What to explain:

- Inventories hooks with `cub-scout map hooks --file`
- Scans for lifecycle hazards with `cub-scout scan --file --lifecycle-hazards`
- Writes results to `sample-output/`

GUI now: No GUI checkpoint — this is file-based scanning.

GUI gap: No ConfigHub integration for hook analysis.

GUI feature ask: Hook hazard analysis in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
jq '.hooks[] | {name, mappedPhase}' sample-output/hooks.json
jq '.lifecycleHazards.summary' sample-output/lifecycle-scan.json
jq '.lifecycleHazards.findings[] | {rule, resource}' sample-output/lifecycle-scan.json
jq '.static.findings[] | {resource_name, severity}' sample-output/lifecycle-scan.json
```

What to explain:

- `map hooks` proves how Helm and Argo hooks are interpreted
- `scan --lifecycle-hazards` proves which hook patterns become risky under Argo CD
- Static findings show that lifecycle hazards are one slice of file-based risk detection

GUI now: No GUI checkpoint — this is local evidence.

GUI gap: No visual lifecycle hazard findings dashboard.

GUI feature ask: Migration hazard viewer in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 4: "Cleanup"

Run:

```bash
./cleanup.sh
```

This removes local sample output only.

---

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
