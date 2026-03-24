# Prompts

## Explain First

```text
Read incubator/global-app-layer/bundle-evidence-sample/README.md and contracts.md. Summarize what mutates, what stays read-only, and what evidence proves bundle publication facts, integrity facts, supply-chain facts, and handoff facts.
```

## Run Safely

```text
Work only in incubator/global-app-layer/bundle-evidence-sample. Start with ./setup.sh --explain and ./setup.sh --explain-json | jq. Then run the example end to end, inspect the output JSON and HTML, and summarize what ConfigHub should preserve from this bundle story.
```

## Investigate Failures

```text
The bundle-evidence-sample example failed. Inspect the JSON files under sample-output and explain whether the problem is publication facts, integrity data, supply-chain data, or deployer handoff data.
```
