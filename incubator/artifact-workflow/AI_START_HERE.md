# AI Start Here

Use this page when you want to drive `artifact-workflow` safely with an AI assistant.

## What This Example Is For

This example is for offline inspection and replay of a copied debug bundle.

It reads a bundle fixture and never talks to a live cluster.

It does not mutate ConfigHub.

## Read-Only First

Start here:

```bash
cd incubator/artifact-workflow
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
- `./setup.sh` is read-only with respect to ConfigHub and live infrastructure
- `./verify.sh` is read-only
- `./cleanup.sh` only removes local sample output
- this example never writes ConfigHub state

## What To Verify

```bash
jq '{target, contents}' sample-output/bundle-inspect.json
jq '{summary, findings}' sample-output/bundle-replay-drift.json
jq '{target, gitContext, changes, riskSignals}' sample-output/bundle-summarize.json
```

Use the evidence like this:

- `bundle inspect` proves the captured target and contents summary are preserved
- `bundle replay` proves the drift finding can be re-rendered offline from captured facts
- `bundle summarize` proves the same bundle can be turned into a structured handoff summary with Git context and risk signals

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
