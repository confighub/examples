# Add-on Manager

This is a standalone ConfigHub operational app. It helps an operator review
affected Variants, preview a proposed change, approve the exact scope, run the
allowed action, and read the proof receipt.

## Run Locally

Requires Node.js 18 or newer.

```bash
npm test
npm run verify
node cli.mjs preflight --json
PORT=5173 npm start
```

Then open:

```text
http://localhost:5173
```

## 15-Minute Package Check

This package is meant to make a newly defined operational workflow usable
quickly: scenario, browser GUI, CLI sibling, tests, and proof gaps should all
be visible in one short pass.

```bash
npm run verify
node cli.mjs preflight --json
node cli.mjs map --json
node cli.mjs findings --json
PORT=5173 npm start
```

Use the GUI to confirm the same Variant scope, findings, approval gate, action
state, and proof gaps shown by the CLI. If those surfaces disagree, fix the
shared workflow contract before connecting live operations.

## Connect To ConfigHub

Create a browser OAuth client for this app, then start the app with the server
URL and client ID. Production ConfigHub supports this through `cub
oauthclient`.

```bash
cub auth status
npm run oauth:register
CONFIGHUB_BASE_URL=https://hub.confighub.com OAUTH_CLIENT_ID=<client-id> PORT=5173 npm start
```

`npm run oauth:register` prints the exact `npm start` command with the generated
client ID. It creates or reuses the `add-on-manager-local` OAuth client with the
redirect URI `http://localhost:5173/callback`.

The browser sign-in uses PKCE. When ConfigHub exposes `AuthIssuer`, the app discovers the issuer's OpenID configuration, exchanges the authorization code with the issuer, then exchanges that identity token through ConfigHub. The app calls ConfigHub with `Authorization: Bearer <token>` after sign-in.

After sign-in, the top status chip should show `ConfigHub connected`. If it
only shows `signed in`, the OAuth callback completed but `/api/me` did not
confirm the ConfigHub user/org yet.

## What The App Shows

- Variants grouped by app, space, unit, risk, and next action.
- Plain-English cards explaining how the use case became a workflow, app, and proof path.
- A compact readiness rail for auth, inventory, scope, and operation state.
- The current workflow step in plain English.
- Controls that explain why they are enabled or disabled.
- Approval scope and proof tabs before any live operation is allowed.
- A ConfigHub object or URL gap so the operator can see what authority is in use.

## CLI Sibling

The command-line surface uses the same `data/operational-workflow.json` and
`data/live-bindings.json` files as the browser app:

```bash
node cli.mjs preflight --json
node cli.mjs map --json
node cli.mjs findings --json
node cli.mjs preview --variant <variant-id> --json
node cli.mjs commit --variant <variant-id> --json
node cli.mjs verify --json
node cli.mjs receipt --json
```

The loop is:

```text
preflight -> map/list -> findings -> preview -> approve/commit -> verify -> receipt
```

`commit` means an approved scoped ConfigHub mutation. In this starter package it
stays blocked until the live ConfigHub object, approval object, governed action
executor, proof receipt, and runtime evidence are bound.

## Live Proof Checks

```bash
CONFIGHUB_BASE_URL=https://hub.confighub.com OAUTH_CLIENT_ID=<client-id> npm run oauth:smoke
npm run binding:check
```

`oauth:smoke` checks ConfigHub browser-auth discovery and, when
`CONFIGHUB_ACCESS_TOKEN` is present, `/api/me`. `binding:check` requires
`data/live-bindings.json` to name the live ConfigHub object, approval object,
governed action contract, action endpoint, proof receipt, and runtime evidence
source. The check rejects copied placeholder values from
`data/live-bindings.example.json`.

On a fresh clone, `npm run binding:check` is expected to fail with
`LIVE_BINDINGS_MISSING` until you create deployment-local
`data/live-bindings.json`. That is the correct safe default, not a broken app.

If the ConfigHub org is live but the scenario-specific write path is not ready
yet, bind the app to the verified read surface and make the missing pieces
explicit. For example, the action endpoint can be
`blocked:governed-write-executor-not-installed` and runtime evidence can be
`blocked:no-controller-or-runtime-target-bound`. That makes the GUI useful
without pretending apply is ready.

## Files

- `public/`: browser UI.
- `cli.mjs`: command-line sibling for the same workflow.
- `src/`: local server and workflow loader.
- `data/operational-workflow.json`: the ConfigHub workflow contract used by the app.
- `data/live-bindings.example.json`: the live binding contract to copy when connecting the app, including the governed action contract.
- `data/live-bindings.json`: deployment-local live bindings; intentionally ignored by Git.
- `SPECIFICATION.md`: what the app is allowed to do.
- `JUSTIFICATION.md`: why this workflow deserves an operational app.
- `skills/`: generated assistant skill and trigger-eval stubs for routing future use.
- `tests/`: deterministic local checks.
- `app-export-manifest.json`: export receipt.

## Live Completion Rule

The app is live only when a user signs in through ConfigHub browser OAuth, the
app can call ConfigHub successfully, the affected Variants are bound to real
ConfigHub objects, and the proof receipt reflects the action that ran.
