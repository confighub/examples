# Add-on Manager Sample App

This is a standalone read-only ConfigHub sample app for reviewing platform
add-ons across Variants.

The app runs locally, reads ConfigHub through your existing `cub` login, and
keeps approve/apply actions disabled until a real governed write path is added.

## What It Shows

- Add-ons grouped by Variant.
- ConfigHub spaces, Units, revisions, and Unit data.
- Current chart, app version, and image fields when they are present.
- A simple workflow model: map, preview, approve, apply, prove.
- Explicit proof gaps for approval, controller delivery, runtime state, and
  receipt closeout.

## Requirements

Required:

- Python 3

Optional, for live ConfigHub reads:

- `cub`
- an authenticated `cub` session

Check auth with:

```bash
cub auth status
```

## Run

From this directory:

```bash
python3 serve.py
```

Open:

```text
http://localhost:5173/
```

Stop the server with `Ctrl+C`.

## Verify

From this directory:

```bash
./verify.sh
```

The verification script does not need live ConfigHub access. It checks that the
server starts, the browser UI loads, the workflow endpoint responds, and
mutation requests remain blocked.

## Runtime Configuration

Optional environment variables:

```bash
PORT=5173
CONFIGHUB_BASE=https://hub.confighub.com
```

## Boundaries

This sample is read-only.

It does not prove:

- browser OAuth or PKCE;
- hosted app deployment;
- approval creation;
- apply or mutation;
- controller reconciliation;
- Kubernetes runtime state;
- receipt closeout.

Those are the next gates before this becomes a production operational app.
