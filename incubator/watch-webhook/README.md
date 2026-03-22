# Watch Webhook

This incubator example adapts the `watch-webhook` flow from `cub-scout` into the official `examples` repo.

It shows a small live event-streaming path:

- one local `kind` cluster
- one copied Deployment and Service fixture
- one local webhook receiver
- one `cub-scout watch --webhook ... --once` cycle
- one captured JSONL event stream

## What This Example Is For

Use this example when you want to show how `cub-scout` can push cluster observation events into an external automation or integration endpoint.

This example does not write ConfigHub state.

## Source

This example is adapted from:

- [cub-scout watch-webhook](https://github.com/confighub/cub-scout/tree/main/examples/watch-webhook)

## What It Reads

It reads:

- the copied `fixtures/watch-demo.yaml`
- the current Kubernetes context created by `kind`
- the `cub-scout` binary
- Python 3 for the local webhook receiver

## What It Writes

It writes live infrastructure during setup:

- one local `kind` cluster
- one namespace, Deployment, and Service in `watch-demo`

It writes local verification output under `sample-output/`:

- `webhook-events.jsonl`
- `webhook.stdout.log`
- `webhook.stderr.log`
- `watch.stdout.log`
- `watch.stderr.log`

It does not write ConfigHub state.

## Read-Only First

```bash
cd incubator/watch-webhook
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Quick Start

```bash
./setup.sh
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster, applying the fixture resources, starting a local webhook receiver, and running one `cub-scout watch --webhook --once` cycle.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It checks that the webhook receiver captured JSONL events with the expected schema and at least one `resource.discovered` event for the fixture namespace.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and clears local sample output.

## What Success Looks Like

At the cluster level you should see:

- namespace `watch-demo`
- `Deployment/watch-demo`
- `Service/watch-demo`

At the event level you should see JSONL entries whose `event.type` is one of the current `watch` event types, with at least one `resource.discovered` event in namespace `watch-demo`.

## Evidence To Check

Direct cluster evidence:

```bash
kubectl get all -n watch-demo
```

Webhook evidence:

```bash
./verify.sh
cat sample-output/webhook-events.jsonl
jq -r '.event.type' sample-output/webhook-events.jsonl
```

## Why This Example Matters

This gives the incubator set a practical integration story.

It answers a common question quickly:

- how do we turn live cluster observations into machine-readable events for another system?

That makes it a good companion to the import, topology, and ownership examples because it focuses on outward event delivery instead of local inspection.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
