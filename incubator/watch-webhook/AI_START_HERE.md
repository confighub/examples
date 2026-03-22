# AI Start Here

Use this page when you want to drive `watch-webhook` safely with an AI assistant.

## What This Example Is For

This example is for event delivery from `cub-scout watch` into a local webhook receiver.

It creates a local `kind` cluster, applies a small fixture set, starts a local Python webhook receiver, and runs one `cub-scout watch --webhook --once` cycle.

It does not mutate ConfigHub.

## Read-Only First

Start here:

```bash
cd incubator/watch-webhook
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

```bash
./setup.sh
./verify.sh
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` mutates live infrastructure and writes local event-capture output
- `./verify.sh` is read-only with respect to ConfigHub and live infrastructure
- `./cleanup.sh` deletes the local `kind` cluster and local sample output
- this example never writes ConfigHub state

## What To Verify

```bash
kubectl get all -n watch-demo
jq -r '.event.type' sample-output/webhook-events.jsonl
jq '.event.resource.namespace' sample-output/webhook-events.jsonl
```

Use the evidence like this:

- `kubectl` proves the fixture resources exist
- `watch --webhook --once` proves events can be emitted to an external HTTP endpoint
- the captured JSONL file proves the current event schema and event types that the receiver saw

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
