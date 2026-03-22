# Prompts

## Explain First

```text
Read incubator/watch-webhook/README.md and incubator/watch-webhook/contracts.md. Summarize what mutates live infrastructure, what stays read-only, and what evidence proves that cub-scout watch delivered webhook events without writing ConfigHub state.
```

## Run Safely

```text
Work only in incubator/watch-webhook. Start with ./setup.sh --explain and ./setup.sh --explain-json | jq. Then run the example end to end, verify the webhook JSONL output, and summarize the current event types and schema that were actually captured.
```

## Investigate Failures

```text
The watch-webhook example failed. Inspect sample-output/webhook-events.jsonl, sample-output/watch.stderr.log, and sample-output/webhook.stderr.log. Explain whether the problem is cluster setup, receiver startup, webhook delivery, or a watch event-schema mismatch.
```
