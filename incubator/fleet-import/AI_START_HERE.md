# AI Start Here: fleet-import

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
Read incubator/fleet-import/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This is a multi-cluster aggregation example. No live cluster required.

After the demo, the human will understand:
- How to merge per-cluster import results into one fleet proposal
- Fleet-wide app detection across clusters
- Unified app model suggestions

This example does NOT mutate ConfigHub.
This example does NOT require a live cluster.

## Prerequisites

- `cub-scout` in PATH
- `jq` for JSON inspection

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/fleet-import
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Aggregates fixture import results from multiple clusters
- No cluster needed
- Outputs fleet summary

GUI now: No GUI checkpoint for this stage — this is CLI-only preview.

GUI gap: No visual preview of fleet aggregation.

GUI feature ask: Fleet import preview in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Run The Aggregation" (local files only)

Run:

```bash
./setup.sh
```

What to explain:

- Aggregates per-cluster results into fleet summary
- Writes to `sample-output/`
- No cluster or ConfigHub mutation

GUI now: No GUI checkpoint — this is file-based aggregation.

GUI gap: No ConfigHub integration for fleet rollup.

GUI feature ask: Multi-cluster fleet view in ConfigHub. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
jq '.summary' sample-output/fleet-summary.json
jq '.proposal' sample-output/fleet-summary.json
jq '.summary.byApp | to_entries[] | select(.value | length > 1)' sample-output/fleet-summary.json
```

What to explain:

- `summary` proves the fleet rollup
- `proposal` proves unified app model suggestion
- `byApp` proves which apps span multiple clusters

GUI now: No GUI checkpoint — this is local evidence.

GUI gap: No fleet topology dashboard.

GUI feature ask: Fleet topology view showing apps across clusters. No issue filed yet.

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
