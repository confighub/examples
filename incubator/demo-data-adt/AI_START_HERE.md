# AI Start Here: demo-data-adt

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
Read incubator/demo-data-adt/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This is a scan-first App-Deployment-Target example. No cluster required.

After the demo, the human will understand:
- Immediate risk findings on labeled workload fixtures
- ADT (App-Deployment-Target) model
- Static scanning without live infrastructure

This example does NOT mutate ConfigHub.
This example does NOT require a live cluster.

## Prerequisites

- `cub-scout` in PATH
- `jq` for JSON inspection

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/demo-data-adt
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Scans fixture manifests from local files
- No cluster needed
- Outputs to `sample-output/`

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of scan targets.

GUI feature ask: Scan preview showing target files. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Run The Scan" (local files only)

Run:

```bash
./setup.sh
```

What to explain:

- Scans fixtures and writes results to `sample-output/`
- No cluster or ConfigHub mutation

GUI now: No GUI checkpoint — this is file-based scanning.

GUI gap: No ConfigHub integration for file-based scans.

GUI feature ask: File-based scan upload to ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
jq '.static.findings' sample-output/dev-eshop.scan.json
jq '.static.findings' sample-output/prod-eshop.scan.json
jq '.static.findings' sample-output/prod-website.scan.json
```

What to explain:

- `dev-eshop` proves immediate warnings exist
- `prod-eshop` proves the clean case
- `prod-website` proves same issue can appear in different app/owner paths

GUI now: No GUI checkpoint — this is local evidence.

GUI gap: No visual findings dashboard for file-based scans.

GUI feature ask: Findings viewer in ConfigHub. No issue filed yet.

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
