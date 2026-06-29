# Add-on Manager Sample App

This is a complete standalone ConfigHub sample app for reviewing platform
add-ons across Variants.

It includes the browser app, local API server, fixture data, live ConfigHub read
adapters, tests, and docs. It has no dependency on any external generator
workspace.

## What It Does

- Shows add-ons grouped by Variant.
- Reads ConfigHub spaces, Units, revisions, and Unit data.
- Works without credentials by using bundled fixture data.
- Uses `cub` for live ConfigHub reads when credentials are available.
- Shows approval scope, blocked actions, proof tabs, and receipt preview.
- Keeps mutation endpoints disabled until a real governed write path is added.

## Requirements

- Node.js 20 or newer
- Optional: `cub` and an authenticated ConfigHub session for live reads

Check live access:

```bash
cub auth status
```

## Run The App

From this directory:

```bash
npm start
```

Open:

```text
http://localhost:5173/
```

To force bundled sample data:

```bash
npm run start:fixture
```

## Verify

```bash
npm test
npm run verify
```

The verifier checks that the app starts, the UI loads, fixture data works, live
mutation routes remain blocked, and the sample contains no references to the
tooling that produced it.

## Runtime Configuration

```bash
PORT=5173
DATA_MODE=auto
CONFIGHUB_BASE=https://hub.confighub.com
```

`DATA_MODE` can be:

- `auto`: try live ConfigHub reads first, then fall back to fixtures.
- `live`: require live ConfigHub reads.
- `fixture`: use bundled sample data only.

## Project Layout

```text
public/       browser UI
src/          local API server and ConfigHub adapters
fixtures/     offline sample data
tests/        Node test suite
scripts/      verification script
docs/         architecture, live mode, and testing notes
```

## Boundaries

This app is intentionally safe as a sample. It does not create approvals, apply
changes, or mutate live ConfigHub state. The browser clearly shows those gaps
instead of pretending a preview is a rollout.
