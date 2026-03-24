# AI Start Here

Use this page when you want to drive the stable `artifact-workflow` example safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. say what the GUI shows today
6. say what the GUI does not show yet
7. name the GUI feature ask and cite the issue number if one exists; if not, say that explicitly
8. tell the human to open the GUI and give them time to inspect it
9. ask `Ready to continue?`
10. do not move on until the human says to continue

## Suggested Prompt

```text
Read artifact-workflow/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This example is for offline inspection and replay of a copied debug bundle.

It reads a bundle fixture and never talks to a live cluster.

It does not mutate ConfigHub.

## Stage 1: Preview The Plan (read-only)

```bash
cd artifact-workflow
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI preview for the offline bundle workflow before local output is generated
- GUI feature ask: no issue filed yet for a pre-run bundle inspection preview

Pause after this stage.

## Stage 2: Generate Offline Bundle Outputs (read-only with respect to ConfigHub and live infrastructure)

```bash
./setup.sh
```

Important boundaries:

- `./setup.sh` reads the copied bundle fixture under `fixtures/debug-bundle/`
- it writes only local JSON artifacts under `sample-output/`
- it does not mutate ConfigHub
- it does not mutate live infrastructure

What you should see after:

- `bundle-inspect.json`
- `bundle-replay-drift.json`
- `bundle-summarize.json`

GUI checkpoint:

- GUI now: open `sample-output/bundle-summarize.json` in an editor or JSON viewer to inspect the structured handoff payload
- GUI gap: there is no richer bundle browser or side-by-side replay viewer in this example
- GUI feature ask: no issue filed yet for a GUI bundle inspector or replay viewer

Pause after this stage.

## Stage 3: Verify The Evidence (read-only)

```bash
./verify.sh
jq '{target, contents}' sample-output/bundle-inspect.json
jq '{summary, findings}' sample-output/bundle-replay-drift.json
jq '{target, gitContext, changes, riskSignals}' sample-output/bundle-summarize.json
```

Use the evidence like this:

- `bundle inspect` proves the captured target and contents summary are preserved
- `bundle replay` proves the drift finding can be re-rendered offline from captured facts
- `bundle summarize` proves the same bundle can be turned into a structured handoff payload with Git context and risk signals

GUI checkpoint:

- GUI now: `sample-output/bundle-summarize.json` shows the same offline handoff data another tool could consume
- GUI gap: there is no linked drill-down from the summary back to the underlying replayed finding in a visual inspector
- GUI feature ask: no issue filed yet for a linked bundle-summary explorer

Pause after this stage.

## Stage 4: Cleanup

```bash
./cleanup.sh
```

This removes the local sample output only.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
