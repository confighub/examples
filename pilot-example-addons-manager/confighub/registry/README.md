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
- the declared destination (repo, mode, org, visibility, license, CI, deploy target);
- the registry state machine (`state` + `transitions`) and deregistration marker.

## State Machine

`WATCH -> LIVE -> DEPRECATED -> RETIRED`. Every transition records
`actor`, `evidence`, `timestamp`, `generatorVersion`, and `configHubUrl`.

- Exports start in `WATCH`.
- `LIVE` requires the stage-12 proof layers green: transition with
  `node lifecycle.mjs registry --to LIVE --actor <who> --evidence <proof-receipt-ref> --confighub-url <unit-url> --json`.
- `DEPRECATED` marks a live app for retirement:
  `node lifecycle.mjs registry --to DEPRECATED --actor <who> --evidence <reason> --json`.
- `RETIRED` is reachable only through `node lifecycle.mjs decommission --confirm`,
  which records the decommission receipt and deregisters from the fleet index.

`node lifecycle.mjs registry --json` shows the current state and full
transition history.

## Fleet Index Shape

- Model: `one-unit-per-app`
- Fleet Space: `confighub-app-fleet`
- This app's Unit: `confighub-app-fleet/add-on-manager-fleet-record`

Default pending team ratification (generated-operational-app contract draft, open question 3): one fleet-record Unit per app inside the org fleet Space, so revisions, gates, and decommission stay scoped to a single app. The org-level index is the Space-level view over those Units. The alternative under review is a single per-org index Unit.

One record Unit per app keeps revisions, gates, and decommission scoped to a
single app, and the org-level index stays a query over the fleet Space instead
of a contended shared file.

## Register

```bash
cub space create confighub-app-fleet
cub unit create --space confighub-app-fleet add-on-manager-fleet-record confighub/registry/fleet-record.unit.yaml --change-desc "Register add-on-manager in the app fleet index"
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
