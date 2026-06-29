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
fixtures/     offline sample data
tests/        Node test suite
scripts/      verification script
docs/         architecture, live mode, and testing notes
```

## Boundaries

This app is intentionally safe as a sample. It does not create approvals, apply
changes, or mutate live ConfigHub state. The browser clearly shows those gaps
instead of pretending a preview is a rollout.
