# Prompts

## Explain First

```text
Read connected-summary-storage/README.md and connected-summary-storage/contracts.md. Summarize what mutates, what stays read-only, and what evidence proves that stored connected summaries can be queried and turned into a dry-run Slack digest.
```

## Run Safely

```text
Work only in connected-summary-storage. Start with ./setup.sh --explain and ./setup.sh --explain-json | jq. Then run the example end to end, inspect the JSON outputs, and summarize what the fixture store says about kind-dev and kind-prod.
```

## Investigate Failures

```text
The connected-summary-storage example failed. Inspect sample-output/summary-list-all.json, sample-output/summary-list-kind-dev-prod.json, and sample-output/summary-slack-dry-run.json. Explain whether the problem is fixture shape, summary query behavior, or Slack digest rendering.
```
