# Prompts

## Explain First

```text
Read incubator/platform-example/README.md and incubator/platform-example/contracts.md. Summarize what mutates live infrastructure, what stays read-only, and what evidence proves that Flux-managed and unmanaged resources coexist in the same cluster.
```

## Run Safely

```text
Work only in incubator/platform-example. Start with ./setup.sh --explain and ./setup.sh --explain-json | jq. Then run the example end to end, inspect the map, orphan, and trace outputs, and summarize the mixed-ownership picture.
```

## Investigate Failures

```text
The platform-example failed. Inspect sample-output/flux-status.txt, sample-output/map-list.json, sample-output/orphans.json, and sample-output/trace-podinfo.json. Explain whether the problem is Flux bootstrap, upstream source reconciliation, orphan fixture apply, or cub-scout ownership attribution.
```
