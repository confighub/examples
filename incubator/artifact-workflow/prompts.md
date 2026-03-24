# Prompts

## Explain First

```text
Read incubator/artifact-workflow/README.md and incubator/artifact-workflow/contracts.md. Summarize what mutates, what stays read-only, and what evidence proves that a copied debug bundle can be inspected, replayed, and summarized without cluster access.
```

## Run Safely

```text
Work only in incubator/artifact-workflow. Start with ./setup.sh --explain and ./setup.sh --explain-json | jq. Then run the example end to end, inspect the JSON outputs, and summarize what the bundle says about the deployment, Git context, and image drift.
```

## Investigate Failures

```text
The artifact-workflow example failed. Inspect sample-output/bundle-inspect.json, sample-output/bundle-replay-drift.json, and sample-output/bundle-summarize.json. Explain whether the problem is the bundle fixture, replay rendering, or summary rendering.
```
