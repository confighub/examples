# App Fleet Registry Record

This app is registered in an org-level fleet index so an operator can answer
"what generated apps exist, who owns them, and what state are they in" without
opening each repository.

## The Record

`fleet-record.json` (and its ConfigHub-Unit-shaped sibling
`fleet-record.unit.yaml`) carries:

- app id, name, version, and source scenario;
- generator name, version, and commit (the regeneration pin);
- owner and on-call;
- live-binding status;
- the app's browser OAuth client registration (`oauthClient`): client id
  (a public identifier), owning org, registered redirect URIs, creation and
  rotation timestamps, registration state, and recorded mutations;
- the declared destination (repo, mode, org, visibility, license, CI, deploy target);
- the registry state machine (`state` + `transitions`) and deregistration marker.

## Browser Client Lifecycle

`oauthClient` keeps this app's browser sign-in client as fleet inventory: one
client per app per org, every serving origin registered as a redirect URI, and
every change recorded. The registration state machine is
`unregistered -> registered -> rotated -> revoked`:

| Transition | Command | Refused when |
|---|---|---|
| `unregistered -> registered` | `npm run oauth:register` | the record marks the registration `revoked` |
| `registered/rotated -> rotated` | `node lifecycle.mjs rotate-auth --client-id <new-client-id>` | `unregistered` (`CLIENT_UNREGISTERED`), same client id (`SAME_CLIENT`), `revoked` (`CLIENT_REVOKED`) |
| `registered/rotated -> revoked` | `node lifecycle.mjs decommission --confirm` | — |

Skipped states are typed `BLOCK` results at exit 0. Redirect-URI additions and
changes append entries to `oauthClient.mutations`; nothing edits the
registration silently. The client signs in members of the owning org only —
cross-org access uses the platform's trusted-provider path, never a shared
client id.

## State Machine

`WATCH -> LIVE -> DEPRECATED -> RETIRED`. Every transition records
`actor`, `evidence`, `timestamp`, `generatorVersion`, and `configHubUrl`.

- Exports start in `WATCH`.
- `LIVE` requires the stage-12 proof layers green: transition with
  `node lifecycle.mjs registry --to LIVE --actor <who> --evidence <proof-receipt-ref> --confighub-url <unit-url> --json`.
  The transition also runs the same live-binding readiness rule as
  `npm run binding:check`; a proof-looking evidence string cannot mark the app
  LIVE while `data/live-bindings.json` is missing, placeholder-only,
  incomplete, or contract-invalid.
- `DEPRECATED` marks a live app for retirement:
  `node lifecycle.mjs registry --to DEPRECATED --actor <who> --evidence <reason> --json`.
- `RETIRED` is reachable only through `node lifecycle.mjs decommission --confirm`,
  which records the decommission receipt and deregisters from the fleet index.

`node lifecycle.mjs registry --json` shows the current state and full
transition history.

## Fleet Index Shape

- Model: `one-unit-per-app`
- Fleet Space: `confighub-app-fleet`
- This app's Unit: `confighub-app-fleet/cost-management-app-fleet-record`

Default pending team ratification (generated-operational-app contract draft, open question 3): one fleet-record Unit per app inside the org fleet Space, so revisions, gates, and decommission stay scoped to a single app. The org-level index is the Space-level view over those Units. The alternative under review is a single per-org index Unit.

One record Unit per app keeps revisions, gates, and decommission scoped to a
single app, and the org-level index stays a query over the fleet Space instead
of a contended shared file.

## Register

```bash
cub space create confighub-app-fleet
cub unit create --space confighub-app-fleet cost-management-app-fleet-record confighub/registry/fleet-record.unit.yaml --change-desc "Register cost-management-app in the app fleet index"
```

Re-register after meaningful changes (new version, owner change, binding
status change) by updating the same Unit through the governed path.

## Deregister

`node lifecycle.mjs decommission --confirm` transitions this record to
`RETIRED` with the decommission receipt as evidence, records the
deregistration, updates a local index file named by
`CONFIGHUB_FLEET_INDEX_FILE` when present, and blocks `npm start`. Then apply
the updated record to the fleet Unit through the governed path. Remove delete
gates deliberately before retiring the Unit itself.
