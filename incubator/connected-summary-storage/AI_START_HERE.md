# AI Start Here

Use this page when you want to drive the incubator `connected-summary-storage` example safely with an AI assistant.

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
Read incubator/connected-summary-storage/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This example is for querying stored connected summaries and building a dry-run Slack digest from them.

It reads a copied summary-store fixture and never talks to a live cluster.

It does not mutate ConfigHub.

Because the fixture timestamps are static, the example uses a wide explicit lookback window (`87600h`) instead of the default `24h`.

## Stage 1: Preview The Plan (read-only)

```bash
cd incubator/connected-summary-storage
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI preview for the summary query plan before local output is generated
- GUI feature ask: no issue filed yet for a pre-run connected-summary preview

Pause after this stage.

## Stage 2: Generate Summary Outputs (read-only with respect to ConfigHub and live infrastructure)

```bash
./setup.sh
```

Important boundaries:

- `./setup.sh` reads fixture data from `fixtures/summary-store/`
- it writes only local JSON artifacts under `sample-output/`
- it does not mutate ConfigHub
- it does not mutate live infrastructure

What you should see after:

- `summary-list-all.json`
- `summary-list-kind-dev-prod.json`
- `summary-slack-dry-run.json`

GUI checkpoint:

- GUI now: open `sample-output/summary-slack-dry-run.json` in an editor or JSON viewer to inspect the generated digest payload
- GUI gap: there is no richer digest preview or summary browser in this example
- GUI feature ask: no issue filed yet for a GUI summary explorer or Slack-payload preview

Pause after this stage.

## Stage 3: Verify The Evidence (read-only)

```bash
./verify.sh
jq '{count, entries}' sample-output/summary-list-all.json
jq '{count, entries}' sample-output/summary-list-kind-dev-prod.json
jq '{text, blocks}' sample-output/summary-slack-dry-run.json
```

Use the evidence like this:

- the unfiltered summary list proves the stored records are discoverable by time window
- the filtered list proves cluster and namespace narrowing works
- the Slack dry run proves the same stored records can be turned into a machine-readable digest payload without posting to a webhook

GUI checkpoint:

- GUI now: `sample-output/summary-slack-dry-run.json` shows the digest text and structured block payload that would be sent onward
- GUI gap: there is no side-by-side view that links the digest back to the underlying stored summary records
- GUI feature ask: no issue filed yet for a linked summary-to-digest inspector

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
