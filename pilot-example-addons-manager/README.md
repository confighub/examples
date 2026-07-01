# Add-on Manager Sample App

This is a complete standalone ConfigHub sample app for reviewing platform
add-ons across Variants.

It includes the browser app, local static server, fixture data, browser OAuth
login, direct ConfigHub API reads, tests, and docs. It has no dependency on any
external generator workspace.

## What It Does

- Shows add-ons grouped by Variant.
- Reads ConfigHub spaces, Units, revisions, and Unit data.
- Works without credentials by using bundled fixture data.
- Uses browser OAuth for live ConfigHub reads when an OAuth client is registered.
- Shows approval scope, blocked actions, proof tabs, and receipt preview.
- Keeps mutation endpoints disabled until a real governed write path is added.
- Shows whether live ConfigHub object, approval, action, proof, and runtime
  bindings are present.
- Shows a governed action contract so the read-only app can say exactly which
  operation layers still need to be bound before Apply can be enabled.

## Requirements

- Node.js 20 or newer
- Optional: a `cub` build that includes `oauthclient`, used only to register the
  browser app client

The `oauthclient` command is currently on the ConfigHub feature branch from PR
4665. Until that command is released, build `cub` from that branch before
registering the app:

```bash
cd /path/to/confighub
git fetch origin pull/4665/head:oauthclient-registration
git checkout oauthclient-registration
make build-cli
bin/cub auth login --server https://pr-4665.testhub.confighub.net
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

## Run With Browser OAuth

Register this browser app against the ConfigHub server. The redirect URI must
match the local app URL:

```bash
export CONFIGHUB_BASE=https://pr-4665.testhub.confighub.net
export OAUTH_CLIENT_ID=$(/path/to/confighub/bin/cub oauthclient create addon-manager \
  --redirect-uri http://localhost:5173/ -o jq='.ClientID')
CONFIGHUB_BASE=$CONFIGHUB_BASE OAUTH_CLIENT_ID=$OAUTH_CLIENT_ID npm start
```

Open the app, choose `Browser OAuth`, click `Sign in`, and then click
`Call /api/me`. After sign-in, inventory reads go directly from the browser to
the ConfigHub API with the minted browser token.

Check OAuth discovery from the command line:

```bash
CONFIGHUB_BASE=$CONFIGHUB_BASE OAUTH_CLIENT_ID=$OAUTH_CLIENT_ID npm run oauth:smoke
```

After the app is bound to real live operation objects, copy
`data/live-bindings.example.json` to `data/live-bindings.json`, fill in the
ConfigHub object, approval object, action endpoint, proof receipt, and runtime
evidence source, then run:

```bash
npm run binding:check
```

The binding file also carries `action.contract`, which names the operation,
scope fields, required proof layers, and mutation policy. Until that file exists
and passes, the GUI correctly reports missing live bindings and Apply remains
blocked. If the file passes but the action endpoint or runtime evidence is
still marked `blocked:*`, the GUI reports a read-only live surface rather than
pretending preview is a rollout.

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
CONFIGHUB_BASE=https://hub.confighub.com
OAUTH_CLIENT_ID=<registered-browser-client-id>
```

## Project Layout

```text
public/       browser UI and browser OAuth helper
src/          local static server and fixture endpoints
data/         live binding template for real operation wiring
fixtures/     offline sample data
tests/        Node test suite
scripts/      verification script
docs/         architecture, live mode, and testing notes
```

## Boundaries

This app is intentionally safe as a sample. It does not create approvals, apply
changes, or mutate live ConfigHub state. The browser clearly shows those gaps
instead of pretending a preview is a rollout. The next product step is to bind
the action contract to an official ConfigHub governed-action primitive and
controller/runtime proof source.
