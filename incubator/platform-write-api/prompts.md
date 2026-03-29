## Orient Me First

Read this example, do not mutate anything yet, and explain:

- what stack it is for
- what it reads
- what it writes
- what the three routing outcomes are
- what success should look like

Use `./setup.sh --explain`, `./setup.sh --explain-json | jq`, and `./compare.sh --json | jq` first.

## Safe Walkthrough

Guide me through `platform-write-api` stage by stage.

Before each command:

- explain what it does
- say whether it mutates ConfigHub or only reads fixtures/state
- tell me what success looks like
- tell me what the GUI shows today and what it still cannot show
- pause until I say continue

## Prove The Write API

Show me the narrowest proof that ConfigHub is a write API for operational config.

Use:

- `./mutate.sh --explain`
- `./mutate.sh`
- `./compare.sh`
- `./refresh-preview.sh prod`

Explain what the `*` marker proves and what it does not prove.

## Compare The Three Routes

Walk me through apply-here, lift-upstream, and block/escalate without mutating anything except where I explicitly approve it.

Use the machine-readable contracts where possible and call out the current product gaps honestly.
