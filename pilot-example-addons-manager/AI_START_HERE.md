# AI Guide: add-on-manager

A generated ConfigHub operational app for installing, upgrading, reviewing,
and proving platform add-ons through governed delivery paths. Unlike the
seed-and-verify examples, this is a UI app with a zero-dependency CLI sibling
and no `setup.sh` — everything below runs on a cold clone with no install and
no ConfigHub account until the final stage. The live browser GUI is built on
the ConfigHub Custom UI Apps JavaScript SDK (`@confighub/react-auth` +
`@confighub/api`); the neighbouring [`../promoter`](../promoter) example is a
live read/write SPA on the same SDK.

## CRITICAL: Demo Pacing

Run ONE stage at a time. After each stage, STOP and wait for the human before
continuing. Do not run ahead, even if the next command is obvious. The human
may want to inspect the output or the GUI between stages.

## Suggested Prompt

> Walk me through the add-on-manager example stage by stage. Pause after each
> stage so I can look around. Start with the offline checks — I have not
> connected ConfigHub yet.

## Stage 1: Prove the package on a cold clone

No `npm install` needed — the tests, verifier, and CLI use node builtins only.

```bash
npm test                         # all surfaces read the same workflow contract
npm run verify                   # layout + content hygiene, then the tests
node cli.mjs preflight --json    # readiness before any risk
```

Expected: tests pass, and preflight returns `"verdict": "WATCH"` with
`"reason": "LIVE_BINDINGS_MISSING"` — the correct safe default before any live
binding exists. See [contracts.md](contracts.md) for every stable field.

**PAUSE.** Wait for the human.

## Stage 2: Walk the CLI loop to its typed blocker

```bash
node cli.mjs map --json
node cli.mjs findings --json
node cli.mjs preview --variant add-on-manager-dev --json
node cli.mjs commit --variant add-on-manager-dev --json
npm run binding:check
```

Expected: `map` shows the three Variants and their ConfigHub bindings;
`findings` lists the live-binding gap plus the stop rules; `preview` reports
`"mutation": "none"`; and `commit` refuses with
`{"verdict": "BLOCK", "reason": "APPROVED_CONFIGHUB_MUTATION_REQUIRED"}` at
exit 0. That refusal is the point: commit means an approved scoped ConfigHub
mutation, not a CLI flag being accepted.

**PAUSE.** Wait for the human.

## Stage 3: The offline GUI harness

```bash
PORT=5173 npm start              # then open http://localhost:5173
```

This serves `public/` — the offline contract harness, not the primary UI. Use
it to confirm the browser surface shows the same Variant scope, findings,
approval gate, action state, and proof tabs as the CLI in Stage 2. If the two
surfaces disagree, the shared workflow contract is broken; stop and fix that
first.

GUI gap: the harness shows the six proof tabs as `waiting`, but there is no
side-by-side view proving that GUI state and CLI JSON came from the same
`data/operational-workflow.json` read.

GUI feature ask: a "contract parity" panel that renders the live CLI JSON next
to each GUI card so drift between surfaces is visible at a glance.

**PAUSE.** Wait for the human.

## Stage 4: Go live with the SDK GUI (needs a ConfigHub org)

```bash
cub auth status
npm run oauth:register           # prints the exact ui:dev command with your client id
npm install
VITE_CONFIGHUB_BASE_URL=https://hub.confighub.com VITE_OAUTH_CLIENT_ID=<client-id> npm run ui:dev
```

Sign in through the browser OAuth flow, then prove the auth surface and
bindings from the CLI side:

```bash
CONFIGHUB_BASE_URL=https://hub.confighub.com OAUTH_CLIENT_ID=<client-id> npm run oauth:smoke
npm run binding:check
```

Expected: `oauth:smoke` confirms discovery (and `/api/me` when a token is
present); `binding:check` stays `WATCH` until you create a deployment-local
`data/live-bindings.json` from `data/live-bindings.example.json` with real,
non-placeholder values.

**PAUSE.** Wait for the human.

## Stage 5: The app's own life

```bash
node lifecycle.mjs install --json
node lifecycle.mjs registry --json
node lifecycle.mjs upgrade --json
```

Expected: `install` records local state under `.lifecycle/`; `registry` shows
the fleet record in `WATCH` with its state machine
(`WATCH -> LIVE -> DEPRECATED -> RETIRED`); `upgrade` reports what a
regeneration would change. Every result carries the same typed
`verdict`/`reason` contract as Stage 2.

**PAUSE.** Wait for the human.

## Cleanup

Nothing to clean up in ConfigHub — no stage above mutates it. Locally,
remove `.lifecycle/` and `data/live-bindings.json` to return to the cold-clone
state.
