# Cost Management App

An example of the full governed loop: find config-derived waste across an
org, price it against a declared rate card, and turn one finding at a time
into an approved, revision-verified, receipted ConfigHub change. The
neighbouring [`cost-estimator`](../cost-estimator) example is the enforcement
plane for the same problem (price book, budget verdicts, apply gate); this
app is the reduction plane. Start with [AI_START_HERE.md](AI_START_HERE.md)
for a staged walkthrough and [contracts.md](contracts.md) for the stable
outputs automation can assert against.

This is a standalone ConfigHub operational app. It helps an operator review
affected Variants, preview a proposed change, approve the exact scope, run the
allowed action, and read the proof receipt.

Original request:

```text
cost optimisation across our Kubernetes fleet
```

## Run Locally

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

The live browser GUI uses the ConfigHub Custom UI Apps JavaScript SDK: `@confighub/react-auth` for sign-in and `@confighub/api` for typed ConfigHub API calls. Do not reimplement browser OAuth in the app.

Create a browser OAuth client for this app, install the UI dependencies, then start the SDK-backed GUI with the server URL and client ID. Production ConfigHub supports client registration through `cub oauthclient`.

```bash
cub auth status
npm run oauth:register
npm install
VITE_CONFIGHUB_BASE_URL=https://hub.confighub.com VITE_OAUTH_CLIENT_ID=<client-id> npm run ui:dev
```

`npm run oauth:register` prints the exact SDK UI command with the generated client ID. It creates or reuses the local OAuth client with the redirect URI `http://localhost:5173/`, and records the registration (client id, redirect URIs, timestamps, state) in `confighub/registry/fleet-record.json`.

The SDK discovers ConfigHub auth settings from `{{baseUrl}}/api/info`, runs OIDC Authorization Code with PKCE, exchanges the IdP token for a ConfigHub-minted token, keeps that token in memory, and calls ConfigHub with `Authorization: Bearer <token>`.

`PORT=5173 npm start` still runs the local workflow harness and CLI parity server. Use `npm run ui:dev` for the live browser-auth GUI.

## Session Lifecycle

After sign-in the minted ConfigHub token lives in memory only; the SDK never writes token material to browser storage. A page refresh still keeps the user signed in: the auth provider silently re-authenticates through the identity provider's session (an OIDC `prompt=none` redirect) and returns to the page the user was on. A failed silent attempt is never an auth error — the app just settles on the sign-in screen.

Silent re-auth is handled entirely by `@confighub/react-auth` 0.2.0 or later. This app pins `^0.1.0` because 0.1.1 is the newest version published to npm; raise the pin to `^0.2.0` once it is published — the provider owns the whole lifecycle, so no app changes are needed. Until then a page refresh lands on the sign-in screen, and while the identity provider session is still alive the Sign in click completes without a password prompt.

Keep the provider in charge: app code must not persist tokens anywhere, must not clear browser storage on load, and the app must load as a top-level page (silent re-auth cannot complete inside a cross-site iframe). `tests/ui-tool.test.mjs` enforces the storage rule; `logout()` is the only sign-out path.

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
executor, proof receipt, and runtime evidence are bound. That blocker is a typed
`{"verdict": "BLOCK"}` at exit 0, not a shell failure — every CLI, lifecycle,
and binding-check result follows the same rule. See **Result Contract** in
`SPECIFICATION.md` for the full `verdict`/`reason` table; branch on those fields,
not on shell success.

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

On a fresh clone, `npm run binding:check` classifies the state as
`{"verdict": "WATCH", "reason": "LIVE_BINDINGS_MISSING"}` and exits 0 until you
create deployment-local `data/live-bindings.json`.
That is the correct safe default, not a broken app: the check exits non-zero
only if the file exists but cannot be parsed. Every generated CLI and lifecycle
blocker follows the same rule — expected `BLOCK`/`ASK`/`WATCH` outcomes are
typed JSON at exit 0; non-zero is reserved for malformed input, missing core
files, or a runtime refusal (a decommissioned `npm start`).

If the ConfigHub org is live but the scenario-specific write path is not ready
yet, bind the app to the verified read surface and make the missing pieces
explicit. For example, the action endpoint can be
`blocked:governed-write-executor-not-installed` and runtime evidence can be
`blocked:no-controller-or-runtime-target-bound`. That makes the GUI useful
without pretending apply is ready.

## ConfigHub Self-Management

This operational app is also a ConfigHub-managed app. Its own config,
Variants, OAuth settings, deployment manifests, promotions, lifecycle,
security, and fleet placement belong in ConfigHub too.

The starter self-management pack lives under `confighub/`:

- `confighub/self-management.json`: app config, Variant, delivery, promotion,
  lifecycle, security, fleet, and proof contract.
- `confighub/variants/*.json`: ConfigHub app config Units for dev, stage, and
  prod-style Variants.
- `confighub/k8s/*.yaml`: Kubernetes manifests to govern and deliver through
  ConfigHub OCI GitOps.

The live app is not complete until those app-self-management artifacts are
created as ConfigHub Units, promoted through real Variants, delivered through
`ArgoCDOCI` or `FluxOCI`, reconciled by the controller, and proven running on
Kubernetes.

## App Lifecycle

The workflow loop above is the app's job. The commands below are the app's own
life. Every export ships all six:

```bash
node lifecycle.mjs install --json
node lifecycle.mjs upgrade [--from <regenerated-dir>] [--apply] --json
node lifecycle.mjs migrate --json
node lifecycle.mjs rollback [--to <backup-stamp>] --json
node lifecycle.mjs rotate-auth --client-id <new-client-id> --json
node lifecycle.mjs decommission --confirm --json
```

- `install` verifies the app layout, records local install state under
  `.lifecycle/`, and refreshes the fleet record's binding status.
- `upgrade` diffs a regeneration against local modifications. The generator
  name and version are pinned in `app-export-manifest.json`; regenerate with
  the same or a newer generator into a separate directory, then run
  `upgrade --from <dir>`. Local-only edits are preserved, upstream-only
  changes apply cleanly with `--apply`, and files changed on both sides are
  reported as conflicts and never overwritten.
- `migrate` applies schema-version migrations for
  `data/operational-workflow.json`, `data/live-bindings.json`, and
  `confighub/self-management.json`. The supported schema versions are pinned
  in the manifest's `schemas` block.
- `rollback` restores the most recent lifecycle backup (taken automatically
  before any lifecycle mutation). The app's ConfigHub-side config rolls back
  separately by restoring the app config Unit revision through the governed
  path.
- `rotate-auth` records an OAuth client rotation in the fleet record: the
  client moves to the `rotated` state with the new client id, `lastRotatedAt`,
  and an appended mutation entry. Rotating an app whose client was never
  registered is a typed `BLOCK` (`CLIENT_UNREGISTERED`) — register first with
  `npm run oauth:register`. After rotating, update the deployment secret
  through the governed config path, restart, then prove with
  `npm run oauth:smoke` and a fresh sign-in before revoking the old client.
- `decommission` (with `--confirm`) retires the app: the fleet record
  transitions to `RETIRED` with the decommission receipt as evidence,
  `npm start` is blocked, an index file named by `CONFIGHUB_FLEET_INDEX_FILE`
  is deregistered when present, and the fleet-record Unit change is applied
  through the governed path. `rollback` reverses it.

## Fleet Registry

This app is registered in an org-level fleet index at export time. The record
lives at `confighub/registry/fleet-record.json` with a ConfigHub-Unit-shaped
sibling at `confighub/registry/fleet-record.unit.yaml`: app id, version,
generator version, owner, on-call, binding status, and destination. The record
carries the registry state machine `WATCH -> LIVE -> DEPRECATED -> RETIRED`;
every transition records actor, evidence, timestamp, generator version, and
ConfigHub URL (`node lifecycle.mjs registry --json` to inspect, `--to` to
transition; `LIVE` requires the proof layers green). See
`confighub/registry/README.md` for the registration commands and the fleet
index shape decision.

## Browser Client Fleet Policy

Generated apps multiply, and so do their browser sign-in clients. The fleet
record owns this app's client lifecycle so the client never becomes an
untracked side effect:

- **One client per app per org.** `npm run oauth:register` creates or reuses
  this app's client and records the registration in the `oauthClient` block of
  `confighub/registry/fleet-record.json`.
- **Client ids are public identifiers, but registrations are inventory.** The
  record tracks the client id, owning org, registered redirect URIs, creation
  time, last rotation, and the registration state machine
  `unregistered -> registered -> rotated -> revoked`.
- **Every serving origin must be a registered redirect URI.** Adding or
  changing a redirect URI is a recorded mutation in the fleet record, never a
  silent edit.
- **Rotation and decommission leave records.** `rotate-auth` refuses to skip
  states (rotating an unregistered client is a typed `BLOCK` at exit 0), and
  `decommission --confirm` marks the registration `revoked` with a recorded
  mutation.
- **Nothing multi-org through this client.** The client signs in members of
  the owning org only; cross-org access goes through the platform's
  trusted-provider path, not through sharing this app's client id.

## Files

- `public/`: browser UI.
- `ui/`: SDK-backed React browser GUI built with `@confighub/react-auth` and
  `@confighub/api`; this is the default live UI tool for ConfigHub apps.
- `cli.mjs`: command-line sibling for the same workflow.
- `lifecycle.mjs`: the app's own life — install, upgrade, migrate, rollback, rotate-auth, decommission.
- `src/`: local server and workflow loader.
- `data/operational-workflow.json`: the ConfigHub workflow contract used by the app.
- `data/live-bindings.example.json`: the live binding contract to copy when connecting the app, including the governed action contract.
- `data/live-bindings.json`: deployment-local live bindings; intentionally ignored by Git.
- `confighub/`: self-management pack for this app's own ConfigHub config,
  Variant overlays, and CH-OCI-GitOps Kubernetes delivery.
- `confighub/registry/`: fleet-record for the org-level app registry, in JSON
  and ConfigHub-Unit-shaped YAML, plus registration commands.
- `SPECIFICATION.md`: what the app is allowed to do.
- `JUSTIFICATION.md`: why this workflow deserves an operational app.
- `skills/`: generated assistant skill and trigger-eval stubs for routing future use.
- `tests/`: deterministic local checks.
- `app-export-manifest.json`: export receipt.

## Live Completion Rule

The app is live only when a user signs in through ConfigHub browser OAuth, the
app can call ConfigHub successfully, the affected Variants are bound to real
ConfigHub objects, and the proof receipt reflects the action that ran.

## Cost engine

`npm run cost:sweep` runs two read-only, org-wide ConfigHub queries (a CEL
extraction of every Deployment/StatefulSet container's replicas and resources,
and a Unit listing with Target bindings), then writes priced findings to
`data/cost-findings.json` (deployment-local, gitignored). The rules live in
`src/cost-engine.mjs` and hold three honesty lines that the tests enforce:

- Requests drive node provisioning, so only request-backed numbers are priced.
  Limits are exposure, never savings.
- A Unit with no Target binding gets no runtime-cost claim; it is reported as
  configured-cost-only and excluded from the savings total.
- Every priced figure carries the rate-card basis from `data/rate-card.json`.

Findings surface in `cli.mjs findings`, `/api/cost-findings`, and the signed-in
GUI. Every recommendation is a governed dry-run diff
(`cub function set ... --dry-run -o mutations`); nothing mutates without the
approval gate.
