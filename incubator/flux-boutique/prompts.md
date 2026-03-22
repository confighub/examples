# Prompts

## Explain First

```text
Read incubator/flux-boutique/README.md and incubator/flux-boutique/contracts.md. Summarize what mutates live infrastructure, what stays read-only, and what evidence proves that one GitRepository fans out into many Flux-managed services.
```

## Run Safely

```text
Work only in incubator/flux-boutique. Start with ./setup.sh --explain and ./setup.sh --explain-json | jq. Then run the example end to end, inspect map-list.json and trace-payment.json, and summarize what the current Flux ownership fan-out looks like.
```

## Investigate Failures

```text
The flux-boutique example failed. Inspect sample-output/map-list.json, sample-output/trace-payment.json, and sample-output/flux-kustomizations.txt. Explain whether the problem is Flux bootstrap, source readiness, workload rollout, or cub-scout ownership attribution.
```
