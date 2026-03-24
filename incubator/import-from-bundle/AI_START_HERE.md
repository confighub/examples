# AI Start Here

Use this page when you want to drive the incubator `import-from-bundle` example safely with an AI assistant.

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
Read incubator/import-from-bundle/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This is an offline import proposal example.

It is useful when the human wants dry-run import evidence without cluster access.

It does not require a live cluster.
It does require `cub-scout`.

## Stage 1: Preview The Plan (read-only)

```bash
cd incubator/import-from-bundle
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no bundle-import preview panel in the GUI today
- GUI feature ask: no issue filed yet for a GUI preview of offline import proposals

Pause after this stage.

## Stage 2: Generate The Dry-Run Proposal (writes local files only)

```bash
./setup.sh
```

Important boundaries:

- `./setup.sh` writes local output only
- it does not mutate ConfigHub
- it does not mutate live infrastructure

What you should see after:

- `sample-output/suggestion.json`
- one dry-run proposal based on the bundle fixture

GUI checkpoint:

- GUI now: none; the output is local JSON only
- GUI gap: there is no GUI import proposal viewer for this offline bundle flow
- GUI feature ask: no issue filed yet for a GUI proposal viewer for bundle-backed import

Pause after this stage.

## Stage 3: Verify The Evidence (read-only)

```bash
./verify.sh
jq '.evidence' sample-output/suggestion.json
jq '.proposal' sample-output/suggestion.json
jq '.workloads' sample-output/suggestion.json
```

Use the evidence like this:

- `evidence` proves the bundle source path story
- `proposal` proves the dry-run import model
- `workloads` proves what the bundle contributed

GUI checkpoint:

- GUI now: none; the verification is CLI- and file-based
- GUI gap: there is no side-by-side viewer for evidence, proposal, and workloads
- GUI feature ask: no issue filed yet for a structured offline import evidence viewer

Pause after this stage.

## Stage 4: Cleanup

```bash
./cleanup.sh
```

This removes local sample output only.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
