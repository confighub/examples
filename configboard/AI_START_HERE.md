# AI Guide: configboard

A BI-style dashboard app over ConfigHub configuration data. Like
[`promoter`](../promoter/AI_START_HERE.md), it is a **webapp with no `setup.sh`** — it
seeds nothing and reads whatever organization you point it at. Built on the published
ConfigHub JS SDK: [`@confighub/rtk-query`](https://github.com/confighub/js-sdk) for
data, [`@confighub/react-auth`](https://github.com/confighub/js-sdk) for browser-direct
OIDC PKCE login.

Read [DESIGN.md](DESIGN.md) before changing query or chart behaviour. It records what
was measured against a real instance, and the decisions that are settled rather than
open.

## Read-only first

```bash
./preflight.sh --json | jq     # what dimensions this org actually has
cub space list -o json
cub target list --space "*" -o json
```

`preflight.sh` mutates nothing. Note `-o json`, not the deprecated `--json`.

## Run it

```bash
cd app
export CLIENT_ID=$(cub oauthclient create configboard-dev \
  --redirect-uri http://localhost:5173/ -o jq='.ClientID')
npm install
cp .env.example .env      # set VITE_OAUTH_CLIENT_ID=$CLIENT_ID
npm run dev
```

Login requires a human at a browser (OIDC redirect to the org's IdP). Do not attempt
to authenticate on the user's behalf. If you need to verify query behaviour without a
browser, use `cub` or call the API with the token from
`~/.confighub/tokens/<context>.json` — that is how the facts in DESIGN.md §2 were
measured.

## Verify without an instance

```bash
cd app
npm test          # aggregation, query compilation, palette, dashboard validation
npm run lint      # tsc --noEmit
npm run gallery   # every chart form against synthetic data
```

`npm test` holds the shipped dashboard YAML to the dimension registry: a typo'd
dimension or an unknown chart form fails, rather than rendering an empty panel. Add a
dimension to `src/query/dimensions.ts` **and** emit it in `src/query/rows.ts` — the
tests catch one without the other.

## Where things are

| Path | What |
|---|---|
| `dashboards/*.yaml` | the dashboards, as data |
| `app/src/model/` | dashboard document types + parser |
| `app/src/query/` | dimension registry, panel→request compiler, aggregation, fetch hooks |
| `app/src/charts/` | chart components + the validated palette |
| `app/src/panels/` | panel chrome and form dispatch |
| `app/src/builder/` | the panel builder (live preview, appends a stanza) |
| `app/src/storage/views.ts` | the seeded Views that provide data-path dimensions |
| `app/src/storage/pinnedDimensions.ts` | recording Triggers + the attach commands |
| `app/src/storage/filters.ts` | save a panel query as a ConfigHub Filter |
| `app/src/dev/` | the chart gallery (dev only) |

## Storage (M1)

Dashboards are `AppConfig/YAML` Units in a `configboard` Space, labelled
`app=configboard`. `src/storage/dashboards.ts` is the only code that writes anything.

```bash
cub unit list --space configboard --where "Labels.app = 'configboard'"
cub revision list <slug> --space configboard      # every save is a revision
cub space delete configboard --recursive           # clean up
```

## Rules that are easy to break

- **`GET /unit` has no pagination.** Never add a unit query without `select`, and never
  select `Data`, `LiveData`, or `LiveState`. A test asserts this.
- **`!= ''` is not a presence test** — it matches rows where the field is absent. Use
  `IS NOT NULL`.
- **Cluster is not a label.** It is the Unit's `Target` (or, at Space grain, the
  Space's `ReleaseTarget`). Do not add a `Cluster` Space label.
- **A Unit with no Target is a category, not missing data** — usually a base unit for
  cloning, or config that is not deployable on its own. Panels that exclude those rows
  say so in the footer.
- **Never cycle categorical hues.** Past the slot ceiling, fold into "Other"
  (`transform.topN`). `palette.test.ts` pins this.
- **Reading must not write.** `list()` must not create the storage Space. Opening the app
  against an untouched org has to mutate nothing.
- **Key any component that seeds state from props.** The source editor and the dashboard
  view both hold state initialized from an entity; unkeyed, React keeps the old state
  when the entity changes. This has produced two real bugs — an unbounded query and a
  save that wrote the wrong unit.
- **Never copy a document `title` straight into `DisplayName`** — the charsets differ.
  Use `toDisplayName()`; the exact title also goes into an Annotation, which accepts any
  character.
- **A View query must include the entities its metadata columns read.** Without
  `include=SpaceID`, a `Space.Labels.*` column returns null with no error.
- **A `.` inside a data-path map key escapes as `~1`** —
  `metadata.annotations.services~1k8s~1aws/region`.
- **Appending a panel appends text.** Re-serializing the document would drop the comments
  a hand-edited dashboard carries.
- **Never widen the write footprint beyond the `configboard` Space.** Attaching a Trigger
  to a workload Space is an operator action; generate the command, do not run it.
- **Findings are recorded state, recomputed by a data revision.** While the
  `--refresh-triggers` re-evaluation bug is open, findings can lag after a Trigger is
  demoted, disabled, or newly matched; `set-annotation` (a data write) recomputes them, a
  Unit `--label` patch does not. An on-demand run of the same check confirms a zero.
- **A `where_trigger` must name one validator.** `FunctionName LIKE 'vet-%'` runs every
  validator over every Unit and does not return. Panels that use it are `manual: true`.
- **`color: status` is only for categories that are states.** On Space slugs every bar
  renders neutral and the chart carries no magnitude.
