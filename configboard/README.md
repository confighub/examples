# configboard — BI-style dashboards over ConfigHub

A browser app that charts the configuration in your ConfigHub organization:
Kubernetes resources, Crossplane and ACK managed resources, app config, and the
change history behind all of them.

The point: because ConfigHub stores configuration as **queryable data** rather than
files in Git, you can point a BI tool at your fleet's desired state and ask questions
a Git repo can't answer — what version of what is where, what is blocked, how long
changes take to land, which config churns most.

Built on the published ConfigHub JS SDK ([confighub/js-sdk](https://github.com/confighub/js-sdk)):
data via [`@confighub/rtk-query`](https://www.npmjs.com/package/@confighub/rtk-query),
browser-direct login via [`@confighub/react-auth`](https://www.npmjs.com/package/@confighub/react-auth).
No backend — it is a static SPA that talks to the ConfigHub API directly.

Like [`promoter`](../promoter/README.md), configboard **seeds nothing**. It is a lens
over whatever organization you point it at. See [DESIGN.md](DESIGN.md) for the full
design, including the milestones beyond what is built.

## Status: M0–M3

Working now:

- Browser-direct OIDC PKCE login against one organization.
- **Dashboards stored as ConfigHub Units.** Each dashboard is one `AppConfig/YAML` unit
  in a `configboard` Space, labelled `app=configboard`. Seed the bundled ones with one
  click, duplicate them, edit the document in the app, delete what you don't want —
  every save is a new revision with a change description, visible in
  `cub revision list`.
- **Cross-filtering.** Click any bar, slice, or heatmap cell to narrow the whole
  dashboard; chips show what's active and remove it.
- Six bundled dashboards: Fleet Overview, Version Skew, Resource Inventory, Fleet
  Posture, Delivery Health, **Compliance**.
- **A findings view.** The `Finding` source explodes each Unit's gate/warning maps into
  one row per failing check, so "which guardrail fires most, and is it live?" is a chart
  costing one unit query. Findings are recorded state, recomputed when a Unit's data
  changes; the on-demand panels let you confirm a zero is current.
- **Compliance panels** that run a named validator server-side and group the failures.
  These are opt-in per panel — a validator sweep costs about 20 seconds regardless of
  scope, so the panel states that and waits to be asked rather than spending it on a tab
  you happened to open. Gates and warnings, already recorded on the Unit, need no sweep
  and load immediately.
- **Pinned dimensions.** Record a config value into `Unit.Values` with a Mutation
  Trigger, where it becomes *filterable* in `where` instead of projection-only. The
  dialog reports which recording Triggers exist in the org and — the part that actually
  matters — how many Spaces **select** each one and how many units those Spaces hold. A
  Trigger nothing selects records nothing and looks exactly like a broken feature.
- **Save as Filter.** Promote a panel's query to a real ConfigHub Filter, usable from
  `cub unit list --filter configboard/<slug>`, a bulk patch, or a Trigger's scope.
- **A panel builder.** Pick a source, dimensions, a measure, and a form; see it
  rendered against your real data before saving. What it saves is the same panel stanza
  a hand-editor would write, appended to the document with comments intact.
- **Data-path dimensions.** Two ConfigHub **Views** (seeded on request) read values out
  of the configuration itself — container image, replica count, cloud region, instance
  class — so dimensions are not limited to metadata. A `derive`/`coalesce` transform
  folds provider-specific spellings (`spec.forProvider.region` vs an ACK annotation)
  into one dimension.
- **Heatmap and histogram**, including the version-skew matrix: component × environment
  with the image tag in the cell.
- **Resource-grain counting** via the read-only `get-resources` function — resources by
  kind, by API group, by provider family (core Kubernetes vs ACK vs Crossplane vs any
  other CRD family), by cluster, and by namespace. A Unit can hold dozens of resources,
  so this is a different question from counting Units.
- Chart forms: stat tile, meter, bar, stacked bar, line, donut, heatmap, histogram,
  plus a table view on every panel.
- Every panel shows the equivalent `cub` command for its query.
- Tier-0 dimensions: Unit / Space / Target / Revision metadata, Space labels,
  Target facts, and the per-Space summary counts.

Not built: custom Attributes (a `get-<slug>` for a path outside the built-in functions),
and "save as View" — only Filters are promotable today. See DESIGN.md §9.

**What it writes.** Nothing, until you ask. Reading never creates the storage Space.
When you seed, duplicate, save, delete, create a View or Trigger, or save a Filter, it
writes only in the `configboard` Space — never a Unit it did not create, and never
anything in a Space it does not own. Making a recording Trigger apply to *your* Spaces
edits those Spaces, so configboard prints those commands rather than running them. Clean
up with `cub space delete configboard --recursive`.

## Prerequisites

- Node 18+.
- [cub CLI](https://docs.confighub.com/get-started/setup/) authenticated
  (`cub auth login`) — to register the OAuth client and to check what your org
  actually has to chart.
- An organization with some configuration in it. Labels matter: see
  [Coverage](#coverage-what-makes-the-dashboards-interesting).

## Run it

```bash
cd app
export CLIENT_ID=$(cub oauthclient create configboard-dev \
  --redirect-uri http://localhost:5173/ -o jq='.ClientID')
npm install
cp .env.example .env          # set VITE_OAUTH_CLIENT_ID=$CLIENT_ID
npm run dev                   # http://localhost:5173
```

Register the client in the **same organization as the app's users** — an app can only
sign in members of the org that owns its `client_id`. See
[Custom UI Apps](https://docs.confighub.com/developer/custom-ui-apps/) for the auth
flow in detail.

`VITE_CONFIGHUB_BASE_URL` defaults to `https://hub.confighub.com`. Point it at your
own instance (e.g. `http://localhost:9090`) if you are running one.

Clean up the throwaway client when you're done:
`cub oauthclient delete configboard-dev`.

## Coverage: what makes the dashboards interesting

configboard slices by **Space labels** and by each Unit's **Target**. An unlabelled
org charts as one bar. Check what you have:

```bash
./preflight.sh          # human-readable
./preflight.sh --json   # machine-readable
```

It reports how many Spaces carry `Component` / `Environment` / `Region`, how many
Units are bound to a Target, and how many Targets have collected `Cluster.*` facts.
Fill gaps with the commands it prints, e.g.:

```bash
cub space update --patch <space> --label Environment=prod --label Component=checkout
```

Note that **cluster is not a label**. A Space's optional `ReleaseTargetID` names the
Target its Units deploy to, and for OCI-delivered Units the Unit's own `TargetID` is
set from it — so the cluster dimension comes from the Target, along with facts like
`Cluster.KubernetesVersion` that `cub` collects rather than a human maintaining.

`ReleaseTargetID` is new, so Spaces created before it exists won't have it set. Until
they do, cluster-grain panels run off each Unit's own `TargetID` (which most Units
have) and the Space-grain panels are sparse. `preflight.sh` reports both counts.

If your org is empty, seed one of these first:
[`promotion-demo-data`](../promotion-demo-data/) (multi-app, multi-environment),
[`initiatives-demo`](../initiatives-demo/) (policies to chart compliance against),
or [`global-app`](../global-app/) (multi-service).

## Dashboards are data

The dashboards are YAML documents in [`dashboards/`](dashboards/), bundled as seed
material; once saved, the stored Units are the source of truth. A panel names a source,
a `where`, a dimension to group by, an aggregate, and a chart form:

```yaml
- id: apply-state
  title: Apply state
  query:
    source: Unit
    where: "Space.Labels.Environment = '${env}' AND Target.Slug = '${cluster}'"
  transform:
    groupBy: Unit.ApplyState
    aggregate: { fn: count }
  chart: { form: bar, orientation: horizontal, color: status }
```

A variable set to **All** drops its whole conjunct from the query rather than matching
the literal string, so an unscoped dashboard runs an unfiltered query.

## How the queries work

ConfigHub does selection, projection, and joins; configboard does grouping,
aggregation, and rendering.

- **`include` is a join.** `include=TargetID` both makes `Target.*` legal in `where`
  and returns the Target expanded on each row — so one request filters *and*
  dimensions across the join. The panel compiler adds the right `include`
  automatically from the dimensions a panel names.
- **`GET /space?summary=true` is the one server-side aggregation.** Per-Space counts
  (total, unapplied, gated, unapproved, upgradable) come back precomputed, so the
  whole KPI row costs zero requests beyond the Space fetch the dashboard already
  makes.
- **`GET /unit` has no pagination**, so unit queries always pass `select` (never
  `Data`, `LiveData`, or `LiveState`) and panels report when a result set is large
  enough to be worth narrowing.

## Development

```bash
npm run dev        # dev server
npm run gallery    # chart gallery: every form against synthetic data, no instance
npm test           # aggregation, query compilation, palette, and dashboard validation
npm run lint       # tsc --noEmit
npm run build      # typecheck + production build
```

`npm test` validates the shipped dashboard YAML against the dimension registry, so a
typo'd dimension or a chart form that doesn't exist fails the build rather than
rendering an empty panel.

The chart palette is validated for colorblind separation and contrast in both light
and dark mode; `src/charts/palette.ts` documents the rules, and `palette.test.ts`
pins the ones that are easy to break.
