# configboard — a BI-style dashboard builder over ConfigHub

**Status:** M0–M3 built.
**Location:** `examples/configboard/`.

## 1. What this is

A browser app that lets a platform engineer **build and share dashboards whose
fact table is ConfigHub itself** — every Kubernetes resource, Crossplane managed
resource, ACK resource, and app-config file across the fleet, plus the change and
delivery history of each one.

The pitch in one line: _because ConfigHub stores config as queryable data rather
than as files in Git, you can point a BI tool at your fleet's desired state and
ask questions that a Git repo can't answer._

What it is **not**:

- Not a metrics/observability dashboard. It charts **configuration** (desired
  state + ConfigHub's own delivery history), not CPU or request rates. Live
  cluster state appears only where ConfigHub already tracks it (`LiveState`,
  Target facts).
- Not a mutation surface for *your* configuration. Every panel is read-only, and every
  write goes into configboard's own Space: its dashboard documents, and the
  View / Trigger / Filter entities it offers to create. It never writes a Unit it did not
  create and never edits a Space it does not own — making a recording Trigger apply to a
  workload Space is an operator action, so configboard prints the commands instead
  (§9, M3).
- Not a replacement for the ConfigHub UI's unit list. Panels drill _into_ it —
  every mark deep-links to `{serverURL}/units/{spaceID}/{unitID}`.

## 2. Why ConfigHub is a workable BI backend

| BI concept                  | ConfigHub primitive                                                                                     |
| --------------------------- | ------------------------------------------------------------------------------------------------------- |
| Fact table                  | Units (`GET /unit`, org-wide, no space required)                                                        |
| Event table                 | Revisions (`GET /revision`) — one row per config change, with `CreatedAt`, `LiveAt`, `Source`, `UserID` |
| Dimension table             | Space Labels, Unit Labels, `Target` (the cluster) with its `Labels` and `Facts`, `ToolchainType`, `ProviderType` |
| `WHERE`                     | `where` (SQL-ish, AND-only), `where_data`, `resource_type`, `trigger_filter`                            |
| `JOIN`                      | `include=<FieldID>` — registers a related entity as a `where` prefix *and* returns it expanded on the row |
| Saved query                 | **Filter** entity                                                                                       |
| `SELECT` projection         | **View** entity — `GET /unit?view=<slug>` returns per-unit `ViewColumns: [{Name, Value}]`               |
| Computed column             | View `MetadataExpression` / `DataExpression` (CEL), `DataPath`                                          |
| Materialized derived column | **Attribute** + Mutation **Trigger** → `Unit.Values` (filterable, no reparse)                           |
| `GROUP BY` / aggregate      | server-side **only** along the Space dimension (`GET /space?summary=true`); every other grouping is client-side |

The `GROUP BY` row is the load-bearing architectural fact. Outside the Space-level
summaries, ConfigHub gives you _selection, projection, and joins_ and `configboard`
supplies _grouping, aggregation, binning, and rendering_. A View is very nearly a
`SELECT` list, and `GET /unit?view=…` returns a tabular result set — which is why
the whole tool can be a thin client.

The `JOIN` row matters more than it looks. A related entity is a legal prefix in
the `where` expression — `Space.Labels.X`, `Target.Facts.Y`, `Target.Slug` — and
`include` additionally returns that entity expanded on each row. So one request both
**filters and dimensions** across the join:

```
GET /unit?include=TargetID,SpaceID
        &where=Target.Facts.Cluster.KubernetesVersion LIKE '1.30.%'
               AND Space.Labels.Environment = 'prod'
```

returns prod Units on 1.30 clusters, each row already carrying its Space and
Target. No client-side join, no pre-resolution pass. Fact keys are dotted and
matched up to the operator, so `Facts.Cluster.KubernetesVersion` needs no quoting.

Three things measured against a real 56-Space / 398-Unit organization, worth knowing
before building on this:

- **Filtering across a join does not require `include`.** `where=Target.Slug = '…'`
  filters correctly on its own; `include` governs whether the entity comes back on the
  row. configboard still adds the `include` whenever a panel *reads* the joined value
  (as a dimension or a drill-down column) — which is most of the time, since a row
  that can't name its own Space is a row you can't drill into.
- **A joined filter is not measurably more expensive.** `Space.Labels.Environment =
  'prod'` narrowed 398 Units to 116 in less wall-clock time than the unfiltered
  baseline. Pushing the predicate down beats fetching everything and filtering in the
  browser.
- **`!= ''` is not a presence test.** `where=Target.Slug != ''` returned all 398 rows,
  including Units with no Target at all. Use `IS NOT NULL` (`Target.Facts.Cluster.KubernetesVersion IS NOT NULL`),
  which returns 0 when no Target carries the fact — correctly. A panel that "filters"
  with `!= ''` silently charts everything, which is the worst failure mode available
  to a BI tool.

### Known limits to design around

- **No general server-side aggregation, no `OR`, no arbitrary `GROUP BY`.** All
  `where` clauses are conjunctions. A panel that needs a union of conditions issues
  N queries and merges client-side, or filters the superset in the browser. The
  one exception is the per-Space summary rollup (§4, Tier 0½), which the query
  compiler should prefer whenever it can answer the panel.
- **No pagination on `GET /unit`.** You get the whole result set. Mitigation:
  always pass `select` to drop the heavy fields (`Data`, `LiveData`,
  `LiveState`), scope by Space labels, and enforce a row budget (§8).
- **`Data.` / `where_data` paths reparse config on every query.** Fine for
  exploration; promote hot dimensions to `Values` via Attribute+Trigger (§4).
- **`Unit.Values` are strings.** `Values.Replicas/replicas > '3'` is a _string_
  comparison. Numeric dimensions should be cast client-side, and the builder
  should warn when a string-compared filter is used on a numeric measure.

## 3. Concepts → data model

Dashboards are stored the way `promoter` stores workflows: as ConfigHub Units, in
the tool's own Space. The BI tool's own artifacts are config-as-data — a small
piece of dogfooding that costs nothing and demonstrates the model.

| configboard concept     | ConfigHub                                                                     |
| -------------------- | ----------------------------------------------------------------------------- |
| **Dashboard**        | one `AppConfig/YAML` Unit in the `configboard` Space, labelled `app=configboard`    |
| **Panel**            | a stanza in that document                                                     |
| **Query**            | a `where` (+ optional saved Filter) + a View (saved or inline)                |
| **Dimension**        | a View column: metadata attribute, Values key, CEL expr, or DataPath          |
| **Scope / variable** | a dashboard-level control that injects an `AND` clause into every panel query |

### Dashboard document

```yaml
apiVersion: configboard.confighub.com/v1
kind: Dashboard
title: Fleet Overview
description: Composition, posture, and change velocity across every space.

# The filter row rendered above the panels. Each variable's distinct values are
# discovered live from GET /space?include=ReleaseTargetID (labels, and the
# Target each Space releases to).
variables:
  - name: env
    label: Environment
    from: { spaceLabel: Environment }
    default: prod
    allValue: true # "All" omits the clause entirely
  - name: cluster
    label: Cluster
    from: { target: Slug } # Unit → Target (include=TargetID)
    allValue: true
  - name: window
    label: Time range
    type: timeRange
    default: 30d

panels:
  - id: kinds
    title: Resources by kind
    span: 6
    query:
      source: Unit
      where: "Space.Labels.Environment = '${env}'"
      view: configboard/kind-dims # space/slug of a saved View
    transform:
      groupBy: Kind
      aggregate: count
      topN: 7 # remainder folded into "Other", never a 9th hue
    chart:
      form: bar
      orientation: horizontal
      color: sequential

  - id: change-velocity
    title: Changes per day by environment
    span: 6
    query:
      source: Revision
      where: "CreatedAt > '${window.start}'"
    transform:
      bin: { field: CreatedAt, unit: day }
      groupBy: Space.Labels.Environment
      aggregate: count
    chart:
      form: line
      color: categorical # ≤4 series, direct-labeled
```

`source` is one of `Unit | Revision | Space | Target`.
`transform` ops: `bin` (time bucket), `groupBy` (1–2 keys), `aggregate`
(`count | sum | avg | min | max | p50 | p95 | distinctCount`), `topN` (+Other),
`pivot`, `compareTo` (baseline series for diverging/dumbbell forms), `having`.

### The View is the schema

Views are the reusable half. A dashboard references saved Views by `space/slug`,
so the same projection powers a panel, a `cub unit list --view` in a terminal, and
the ConfigHub UI. Example projection behind the panel above:

```bash
cub view create --space configboard kind-dims --of Unit \
  --column Unit.Slug --column Space.Slug \
  --column Space.Labels.Environment --column Space.Labels.Region
# plus DataPath columns (Kind, Replicas, Image) supplied via JSON — see §4
```

## 4. Making arbitrary resource types chartable

This is the part that decides whether the tool works for Crossplane MRs, ACK
resources, Istio, cert-manager, and whatever the user installed last week.
Three tiers, in increasing cost and capability. The builder UI exposes all three
and nudges you up the ladder.

### Tier 0 — metadata (free, filterable, no config parsing)

On the Unit itself: `Labels`, `ToolchainType`, `ProviderType`, `TargetID`,
`HeadRevisionNum` vs `LastReleasedRevisionNum`, `ApplyGates`, `ApplyWarnings`,
`ApprovedBy`, `UpstreamRevisionNum`, `UpdatedAt`, `LastActionAt`.

One `include` away: `Space.*` (labels), `Target.*` (labels and facts),
`UpstreamUnit.*`, `HeadRevision.*`, `LastAppliedRevision.*`, `ChangeSet.*`.
Filterable and returned expanded, per the `JOIN` row in §2.

The standard Space label set (`Component`, `Environment`, `Region`, `Layer`,
`Variant`) is what makes cross-space slicing work at all — a dashboard over an
unlabelled org is a dashboard with one bar. **The setup script's first job is
labelling.**

**Cluster is not a label — it's a reference, and it joins.** A Space's optional
`ReleaseTargetID` names the Target that is the default destination for its Units,
and for Units delivered via ProviderType `OCI` the Unit's own `TargetID` is set
from it. So the cluster is reachable **directly from the Unit**:
`include=TargetID` puts `Target.Slug`, `Target.Labels.*`, and `Target.Facts.*`
into both the `where` vocabulary and the returned row.

This is strictly better than a label. The Target carries `Labels` *and* `Facts`,
so the cluster dimension yields `Facts.Cluster.KubernetesVersion`,
`StorageClasses`, `IngressClasses`, `ClusterScopedCRDs`, and any custom facts —
real cluster properties, collected by `cub` rather than hand-maintained. A label
would have to be kept in sync by a human; this can't drift.

`Space.ReleaseTarget` (via `GET /space?include=ReleaseTargetID`) remains the right
join for **Space-grain** panels — anything built on the summary counts below,
where the row is a Space and there is no Unit to hang a Target off.

### Resource grain — `get-resources`, one row per resource

A Unit holds one or more resources; a rendered Helm chart holds dozens. So "how many
Deployments do we run?" is not a Unit-list question, and neither is "which CRD families
are in the fleet?". `POST /function/invoke` with the read-only `get-resources` function
answers both:

```
POST /function/invoke?where=Space.Labels.Environment = 'prod'
{ "ToolchainType": "Kubernetes/YAML",
  "FunctionInvocations": [ { "FunctionName": "get-resources",
                             "Arguments": [ { "ParameterName": "body", "Value": "none" } ] } ] }
```

Each element of the response carries `UnitID` / `UnitSlug` / `SpaceSlug` / `TargetID`
plus `Outputs.ResourceList` — base64 JSON of
`{ResourceType, ResourceName, ResourceNameWithoutScope, ResourceCategory}` per resource.
`body=none` omits each resource's body; `native` would return the YAML, which a count
never needs.

`ResourceType` is `[group/]version/Kind`, and the **group is the dimension that makes
this general**: `ec2.services.k8s.aws` is ACK, `*.upbound.io` / `*.crossplane.io` is
Crossplane, `traefik.io` and `cert-manager.io` are whatever the platform team installed.
configboard classifies families by group suffix, so a new CRD family appears in the
charts without configboard knowing it exists.

**The cost is real and worth stating.** Measured org-wide on 393 Units / 1,423
resources: **~31s and a 58 MB response, 32 MB of which was `ConfigData`** — every Unit's
full config body, returned even though `get-resources` is non-mutating and `body=none`
was requested. The server no longer does that: `ConfigData` comes back only when the
invocation changed the configuration, and `DataHash` says so when it did not, which
takes that 32 MB out of this response. The rest of the cost stands, and the mitigations
in the app remain: all resource panels on a dashboard share one invocation (RTK Query
cache), the panel footer states the resource count and suggests narrowing, and the scope
variables push a `where` down so the common case invokes a fraction of the fleet.

### Tier 0½ — Space summaries (server-side rollups, one request)

`GET /space?summary=true` returns per-Space counts computed on the server:
`TotalUnitCount`, `UnreleasedUnitCount`, `UnapprovedUnitCount`, `GatedUnitCount`,
`WarnedUnitCount`, `UpgradableUnitCount`, `UnlinkedUnitCount`, plus `TargetCountByToolchainType`, `TriggerCountByEventType`,
and totals for Links, Filters, Views, Tags, ChangeSets, Invocations, Attributes,
Releases, BridgeWorkers.

This is the one place ConfigHub *does* aggregate for you, and it is exactly the
grain most governance panels want. The whole KPI row (§5D) and any per-Space,
per-Environment, or per-cluster posture chart is **one request over Spaces**, not
a pull of every Unit in the org. Rule for the query compiler: **if a panel groups
by a Space-level dimension and measures one of the summarized counts, compile it
to `GET /space?summary=true` and never touch `/unit`.** Fall through to unit-level
queries only for dimensions the summary doesn't carry.

### Tier 1 — DataPath / DataExpression View columns (projection only)

A View column can pull any path out of the config data, per resource type:

```json
{
  "Name": "Region",
  "ColumnType": "DataPath",
  "DataType": "string",
  "ColumnSource": {
    "DataPath": {
      "Path": "spec.forProvider.region",
      "WhereResource": "ConfigHub.ResourceType = 'ec2.aws.upbound.io/v1beta1/VPC'"
    }
  }
}
```

That is how a Crossplane MR's region, an ACK `DBInstance`'s instance class, or a
`Certificate`'s issuer becomes a chart dimension **without ConfigHub knowing the
CRD**. Path syntax supports `*` wildcards and `?key=value` associative matching,
so `spec.template.spec.containers.?name=main.image` works.

Cost: parsed per query, and **not usable in `where`** (only `Data.`/`where_data`
predicates are, which reparse too). Right for exploration and long-tail types.

### Tier 2 — Attribute + Mutation Trigger → `Unit.Values` (filterable, cached)

Define an Attribute in the Space (`get-<slug>`/`set-<slug>` get registered), then
wire a Mutation Trigger on the getter. Every data change re-records the value into
`Unit.Values["<TriggerSlug>/<attribute>"]`, where it is **filterable and returned
with the plain unit list** — no reparse:

```bash
cub trigger create --space configboard Image    Mutation Kubernetes/YAML get-image "@0"
cub trigger create --space configboard Replicas Mutation Kubernetes/YAML get-replicas
# → Values."Image/container-image", Values."Replicas/replicas"
```

The builder offers **"pin this dimension"** on any Tier-1 column: it creates the
Attribute (for custom paths) and the Trigger, and rewrites the panel to read
`Unit.Values.<key>`. That is the single most instructive interaction in the
example — it shows a user _materializing a derived column_ in a config database.

Caveat worth surfacing in the UI: a newly created Trigger populates `Values` only
on the **next** mutation of each Unit. Backfill = a no-op patch across the
selected units, which is a write — so it's offered explicitly, never implicitly.

### Compliance as a dimension

`trigger_filter` + `triggers_passed` runs validators as part of the query, so
"units failing the platform's `vet-*` triggers" is a first-class panel source
without any precomputation. Pair with `initiatives-demo`'s Kyverno-CEL policies
for a compliance dashboard that is genuinely live.

## 5. Panel catalog — the use cases

Chart forms follow the `dataviz` method: the data's _job_ picks the form, color
comes last, categorical hues are assigned in fixed order and capped at 7 + Other.
Where the user asked for "circle charts," a donut is used only where it is
honestly right — a **part-to-whole with ≤4 slices, or one-vs-rest** — and a
horizontal stacked/plain bar is substituted where a pie would lie. Meters (a
single ratio against a limit) carry the compliance ratios.

### A. Composition — "what do we actually run?"

| Panel                                                                                                       | Query                                                      | Form                                           |
| ----------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------- | ---------------------------------------------- |
| Resources by kind                                                                                           | Unit + View(`Data.kind`), topN 7                           | horizontal bar, sequential                     |
| Managed-resource mix by provider group (`ec2.aws.upbound.io`, `rds.services.k8s.aws`, `sql.gcp.upbound.io`) | Unit + View(CEL over `apiVersion`)                         | horizontal bar, categorical                    |
| **Toolchain share** (`Kubernetes/YAML` vs `AppConfig/YAML`)                                                 | Unit, `groupBy ToolchainType`                              | **donut** (3–4 slices — legitimately circular) |
| Units per Space / per cluster                                                                               | `GET /space?summary=true`, `TotalUnitCount`                | horizontal bar                                 |
| Cloud resources by region                                                                                   | Unit + View(`spec.forProvider.region` / ACK `spec.region`) | horizontal bar                                 |
| Environment × kind                                                                                          | Unit, groupBy(Environment, Kind)                           | stacked bar, categorical                       |

### B. Posture — "is the fleet consistent and safe?"

| Panel                                                        | Query                                                                                           | Form                                                                                      |
| ------------------------------------------------------------ | ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------- |
| **Version skew matrix**: component × environment, cell = image tag | Unit + `Values.Image/container-image`, pivot                                              | **heatmap** (cell colored by "matches prod baseline?" — diverging), values direct-labeled |
| Instance-type spread across Crossplane/ACK MRs               | View(`spec.forProvider.instanceType`)                                                           | horizontal bar (cost proxy)                                                               |
| Replica distribution                                         | `Values.Replicas/replicas`, binned                                                              | histogram                                                                                 |
| Kubernetes versions across clusters                          | `GET /target`, `Facts.Cluster.KubernetesVersion`                                                | bar with **emphasis** — EOL versions in the accent hue, the rest gray                     |
| **Workloads stranded on old clusters**                       | `GET /unit?include=TargetID&where=Target.Facts.Cluster.KubernetesVersion LIKE '1.2%'`           | horizontal bar by component — one request, no join                                        |
| **Posture per cluster**: gated / unapproved / unapplied share | `GET /space?summary=true&include=ReleaseTargetID`, roll Spaces up by `ReleaseTarget.Slug`       | stacked bar, status palette — one request for the whole fleet                             |
| Units missing resource limits / probes / securityContext     | `trigger_filter=platform/standard-vets&triggers_passed=false`, groupBy `Space.Labels.Component` | horizontal bar, status color                                                              |
| Public-ingress / wide-open NetworkPolicy count by env        | `where_data` predicate                                                                          | bar                                                                                       |
| Storage classes in use vs available per cluster              | Unit View(`spec.storageClassName`) ∩ `Facts.Cluster.StorageClasses`                             | dumbbell                                                                                  |

The version-skew heatmap is the panel most worth building first — it is the
question ("what version of what is where?") that a Git-based fleet cannot answer
without cloning N repos, and ConfigHub answers in one request.

### C. Delivery and change — "how fast, how safely?"

| Panel                                                                                                                      | Query                                                                                            | Form                                                 |
| -------------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ | ---------------------------------------------------- |
| Change velocity: revisions/day, one line per environment                                                                   | `GET /revision?where=CreatedAt > …`, bin by day, groupBy `Space.Labels.Environment`              | **line**, ≤4 series, direct-labeled                  |
| **Deploy lead time** (Revision `LiveAt` − `CreatedAt`)                                                                     | `GET /revision`, computed                                                                        | histogram, plus p50/p95 trend line                   |
| Applies landed per day                                                                                                     | `GET /revision?where=LiveAt > …`, bin `LiveAt` by day                                            | column                                               |
| Who/what changes config: `Source` mix (`UpdateUnit`, `PatchUnit`, `Invoke`, `UpgradeUnit`, `CloneUnit`, `RestoreRevision`) | `GET /revision`, groupBy `Source`, bin week                                                      | stacked column — automation-vs-human ratio over time |
| Rollback rate                                                                                                              | Revision `Source = 'RestoreRevision'`                                                            | sparkline in a stat tile                             |
| Change concentration: revisions per unit                                                                                   | `GET /revision`, groupBy `UnitID`, topN 10                                                       | horizontal bar — the churniest config in the fleet   |
| **Unreleased-change aging**: how long changes sit unshipped                                                                 | Unit, `HeadRevisionNum > LastReleasedRevisionNum`, bucket by `UpdatedAt` age (<1d / 1–7d / 7–30d / 30d+) | stacked bar per environment, sequential              |
| Promotion lag: revisions between a downstream unit and its upstream head                                                   | Unit, `UpstreamRevisionNum` vs `UpstreamUnit.HeadRevisionNum`                                    | horizontal bar per app, sequential                   |

### D. Governance KPI row (top of every dashboard)

Stat tiles, not charts — a single current value each, with a delta and sparkline.
The entire row comes from **one** `GET /space?summary=true`, summed across the
Spaces in scope:

| Tile | Summary field |
|---|---|
| **Units under management** (+ 30-day delta) | `TotalUnitCount` |
| **Released and current** — as a **meter** against the total | `TotalUnitCount − UnreleasedUnitCount` |
| **Blocked by ApplyGates** — status-colored | `GatedUnitCount` |
| **Carrying warnings** | `WarnedUnitCount` |
| **Behind upstream** | `UpgradableUnitCount` |
| **Unapproved pending changes** | `UnapprovedUnitCount` |

These are the same populations as the quick filters in the Filters and Views
guide (`LEN(ApplyGates) > 0`, `HeadRevisionNum > LastReleasedRevisionNum`, …), so the tile
drill-down can hand the user the equivalent `--where` expression even though the
count itself came from the rollup. That pairing — cheap rollup for the number,
explicit filter for the drill-down — is the pattern the whole tool should follow.

The two agree exactly, which is what makes the pairing safe to rely on. Measured on a
real org: summed `UnappliedUnitCount` across 56 Spaces = 148, and
`where=HeadRevisionNum > LiveRevisionNum` over the same org returned 148 Units (measured
before those names changed; the pairing is what the measurement was checking, and it holds
the same way for `UnreleasedUnitCount` and `HeadRevisionNum > LastReleasedRevisionNum`). Summed
`GatedUnitCount` = 0 and `LEN(ApplyGates) > 0` returns 0. If those ever diverge, the
tile is lying and the drill-down is right.

### Suggested starter dashboards (shipped as seed Units)

1. **Fleet Overview** — KPI row + composition + version-skew heatmap.
2. **Cloud Footprint** — Crossplane/ACK MRs by provider, region, and instance
   type; the "infra BI" story.
3. **Delivery Health** — velocity, lead time, unapplied-change aging, rollback rate.
4. **Compliance** — per-initiative pass meters, failures by team, trend line.

## 6. Chart library

**Recommendation: [Recharts](https://recharts.org) (MIT) + MUI for chrome.**

Rationale, in the order that decided it:

1. **The other examples are MUI + React 18 + Vite** (`promoter`, `sec-scanner`,
   `rbac-manager`, `cost-estimator`). Matching that keeps the example legible to
   anyone who has read one of the others.
2. **Recharts is declarative SVG composition** — `<Bar>`, `<Line>`, `<Cell>` —
   which means the `dataviz` mark specs (2px lines, 4px rounded data ends, 2px
   surface gap between stacked segments, ≥8px markers) are expressible as props
   rather than fought against. Per-datum `<Cell fill>` is what makes fixed-order
   categorical assignment and emphasis forms straightforward.
3. Custom tooltips and legends are plain React, so the crosshair-and-tooltip and
   direct-label rules are cheap.
4. Small enough to keep the example's dependency story honest (no license
   asterisks, no canvas/WebGL renderer to explain).

Considered and rejected: **MUI X Charts** (tightly integrated but the chart
surface is less malleable, and the community/pro split invites a licensing
question in an example); **ECharts** (more powerful — canvas, huge datasets,
built-in heatmap — but imperative option objects fight the mark specs and it
doubles the bundle); **visx** (right power ceiling, wrong effort budget for an
example); **Observable Plot** (excellent grammar, but React integration is an
escape hatch and the interaction model is ours to rebuild).

The heatmap is Recharts' one gap — it has no first-class heatmap. Build it as a
CSS-grid component (it is a table of colored cells with a legend), which is
simpler than bending a scatter chart into a matrix and gives us real table
semantics for the accessibility fallback.

Concretely: a MUI `Box` with
`gridTemplateColumns: <label-width>px repeat(n, 1fr)`, one styled cell component
at `aspectRatio: 1`, a hover card per cell, a fixed header row above a scrolling
body, and a hand-built legend. Two details worth getting right up front — a
density ladder that thins column labels as the count grows (every label at ≤14
columns, every other at ≤31, weekly beyond), and a hover-scale affordance so small
cells stay clickable.

Color is where hand-rolled heatmaps usually go wrong. Do **not** encode a ratio as
a continuous hue rotation (red→green via `hsl(rate * 140)`): that is the rainbow
encoding the `dataviz` method rules out, and it carries state on hue alone. Use
the diverging blue↔red pair with a gray midpoint, and pair color with a glyph or a
direct label so the reading survives CVD and grayscale printing.

### Visual system

Take the `dataviz` reference palette as-is for the draft (categorical slots
blue → orange → aqua → yellow → magenta → green → violet → red; blue sequential
ramp; blue↔red diverging with a gray midpoint; a reserved status palette), then
swap in ConfigHub brand ramps and re-run `validate_palette.js` before merge.
Non-negotiables carried into the component layer: one y-axis ever, categorical
hues never cycled or generated, legend for ≥2 series with direct labels at ≤4,
status colors never reused as "series 4", table view available for every panel,
dark mode as a selected set of steps rather than an inversion.

## 7. Architecture

Built in M0 (`✓`), planned after it (`·`):

```
examples/configboard/
✓ README.md                 # what it shows, how to run
✓ AI_START_HERE.md          # cold-start protocol (repo convention)
✓ contracts.md              # stable outputs for preflight.sh, the document schema
✓ preflight.sh              # read-only: does this org have the dimensions to chart?
✓ dashboards/*.yaml         # the starter dashboards, as data
  app/
    src/
✓     app/          App, DashboardView, ScopeBar, store, theme, config
✓     model/        types.ts, parse.ts        — the dashboard document
✓     query/        dimensions.ts — the Tier-0 registry
✓                   compile.ts    — panel + scope → request (joins, select, cub cmd)
✓                   execute.ts    — fetch, dedupe, cache, row budget
✓                   rows.ts       — API entities → flat Rows
✓                   aggregate.ts  — bin / groupBy / aggregate / topN fold
✓     charts/       Bar, StackedBar, Line, Donut, StatTile, Meter, DataTable,
✓                   palette.ts (validated), chartTheme.tsx, ChartTooltip
✓     panels/       PanelFrame (query, table toggle, exclusions), PanelRenderer
✓     dev/          Gallery — every chart form against synthetic data
·     charts/       Heatmap, Histogram, Dumbbell
·     builder/      QueryBuilder, DimensionPicker, ChartPicker, PreviewPane
·     storage/      dashboards as Units in the configboard Space
·     dimensions/   pin.ts — create Attribute + Trigger for a Tier-1 dimension
```

**Stack:** React 18 + TypeScript + Vite + MUI + Recharts, on
`@confighub/react-auth` (browser-direct OIDC PKCE) and `@confighub/rtk-query`
(`listAllUnits`, `listAllRevisions`, `listAllTargets`, `listSpaces`,
`listAllViews`, `listAllFilters`). Static SPA, no backend — same
deploy shape as `promoter`. Registration is one command:
`cub oauthclient create configboard --redirect-uri http://localhost:5173/`.

**One prerequisite in the SDK.** The js-sdk pins its spec to `v0.1.90`
(`.spec-version`), whose `Space` schema predates `ReleaseTargetID` / `ReleaseURL` /
`ReleaseBridgeWorkerID`. Bump the pin to the current release (**`v0.2.0`**): edit
`.spec-version`, `npm run sync-spec`, publish. The npm packages are at `0.1.1`
today, so M0 builds against those — it needs only Units, Spaces, Revisions, and
Targets, all long-standing. The Space-grain `ReleaseTarget` panels (§5B) are the
part that waits on the bump.

### Query execution

0. **Fetch the Space spine, once per dashboard.** `GET /space?summary=true&include=ReleaseTargetID`
   yields every Space's labels, its ReleaseTarget, and the summary counts. This
   populates the variable pickers (the distinct Environments, Regions, clusters)
   and answers every Space-grain panel outright. It is *not* a scope-resolution
   workaround — unit-level scoping goes through `include` + `where` on the unit
   query itself.
1. **Compile.** Panel spec → `{ endpoint, where, include, filter, view, select, resource_type }`.
   Dashboard variables become `AND` clauses, using the join prefix when the
   dimension lives on a related entity (`Space.Labels.Environment = 'prod'`,
   `Target.Slug = 'prod-us-east-1'`), and the compiler adds the matching `include`
   automatically. Panels answerable from the step-0 summary compile to no request
   at all. `select` always excludes `Data`, `LiveData`, `LiveState`.
2. **Dedupe.** Panels are keyed by their compiled request; identical requests share
   one fetch (the whole KPI row is _zero_ extra calls — it reads the step-0
   response).
3. **Execute** via RTK Query — caching, dedup, and invalidation come free.
4. **Aggregate** in a pure, unit-testable module: bin → groupBy → aggregate →
   topN/Other → sort. Same module feeds the chart and the table view.
5. **Budget.** Every response reports row count. Past a soft limit (5k rows) the
   panel shows a "narrow the scope" hint with the specific clause to add; past a
   hard limit (25k) it refuses and explains. Honest about the no-pagination limit
   instead of hanging the tab.

### Interaction

- **Cross-filter:** clicking a bar/slice/cell appends `AND <dim> = '<value>'` to
  the dashboard scope; the active clause shows as a removable chip.
- **Drill-down:** a panel's context menu opens the underlying rows as a table,
  each row deep-linking to `{serverURL}/units/{spaceID}/{unitID}`.
- **Show me the query:** every panel can print its equivalent `cub` command
  (`cub unit list --space "*" --where "…" --view configboard/kind-dims`). This is
  what turns the example from a demo into a teaching tool — the chart and the CLI
  are visibly the same query.
- **Save as Filter/View:** promote an ad-hoc panel query into real ConfigHub
  entities, reusable from the CLI and the ConfigHub UI.

## 8. Seed data and the demo story

`configboard` should follow `promoter`'s lead and **not seed workloads** — it is a
lens over whatever org it is pointed at. `setup.sh` seeds only its own Space,
Views, Triggers, and the four starter dashboards, and prints what it found
(spaces, label coverage, resource-type mix) so the user knows whether the org has
enough shape to be interesting.

For a populated org, compose existing examples: `promotion-demo-data` (multi-app,
multi-environment clone chains → velocity and skew panels), `initiatives-demo`
(Kyverno-CEL policies → compliance panels), `eks-manager` (Crossplane MRs →
cloud-footprint panels), `global-app` (multi-service). If none exist, `setup.sh`
should say so and name them rather than silently rendering four empty dashboards.

Dimension coverage is the real prerequisite. `setup.sh --explain` should report
which of `Component` / `Environment` / `Region` are present on Spaces, offer the
exact `cub space update --patch --label …` commands to fill the gaps, and — since
the cluster dimension is a reference rather than a label — report **how many Units
have a `TargetID`**, how many Spaces have a `ReleaseTargetID`, and how many of
those Targets carry collected `Cluster.*` facts (`cub target` fact collection
populates them). A fleet with labels but no Targets loses every per-cluster panel,
which is worth saying out loud before the user opens an empty dashboard.

## 9. Build phases

- **M0 — read-only viewer. Built.** Auth, four starter dashboards as bundled YAML,
  Tier-0 dimensions plus resource grain, six chart forms, table view on every panel,
  the equivalent `cub` command on every panel.
- **M1 — dashboards as data. Built.** Dashboards stored as `AppConfig/YAML` Units in
  the `configboard` Space; seed / duplicate / delete; a source editor that validates
  before it saves; cross-filtering.

  Three things worth recording from building it:

  - **Reading must not write.** `list()` looks the Space up without creating it, so
    opening the app against an org that has never used it mutates nothing. Seeding is
    an explicit button with the write spelled out.
  - **A dashboard title is prose; `DisplayName` is not.** DisplayName must match
    `^[A-Za-z0-9]([\-_ .|A-Za-z0-9]*[A-Za-z0-9.!?])?$`, so a legal title like
    `Fleet Posture (consistency & safety)` fails the save with a raw regex in the error
    body. The title stays authoritative in the document; DisplayName gets a sanitized
    best effort, omitted entirely when nothing legal survives.
  - **Dialogs that hold document text must be keyed to the entity they edit.** The
    source editor keeps the YAML in `useState`; unkeyed, React kept one dashboard's text
    while the props moved to another, and a save wrote the wrong unit. Same failure class
    as the dashboard-scope bug in §7 — any component initializing state from props needs
    a `key`.
- **Cross-filter design:** filters apply **client-side**, after the fetch. Many chart
  dimensions are derived rather than stored (`Unit.ReleaseState`, every `Resource.*`), so
  no `where` clause could express them; pushing only some clicks down would make the
  same gesture mean different things. A filter on a dimension a panel's source lacks is
  ignored *for that panel*, and the panel says so — clicking a resource kind must not
  blank the Space-grain panels beside it.
- **M2 — the builder and Tier-1 dimensions. Built.** A panel builder that previews
  against live data and appends the stanza it produces to the document; Views seeded
  from the app so data-path dimensions work; heatmap and histogram forms; the
  Version Skew dashboard.

  What building it settled:

  - **A View's metadata columns silently return null without `include`.** A column
    reading `Space.Labels.Component` came back empty for all 70 rows until the query
    added `include=SpaceID` — at which point 68 of 70 had a value. Nothing errors; it
    reads as "our Spaces have no labels". A view query now always includes `SpaceID`
    and `TargetID`, because a panel cannot know a saved View's columns.
  - **`view` on the org-wide unit list resolves only a UUID.** A slug has no Space to
    be resolved in, so `view=cb-workload` is rejected. Panels still reference
    `space/slug` (that is what a human writes); the app resolves it to an id first and
    waits for the lookup rather than issuing an unprojected query.
  - **A `.` inside a map key escapes as `~1` in a data path.** ACK writes region into a
    `services.k8s.aws/region` annotation, addressed as
    `metadata.annotations.services~1k8s~1aws/region`. Without that, the panel is empty
    and the reason is invisible.
  - **The same concept needs one dimension across providers.** Region lives at
    `spec.forProvider.region` (Crossplane), `spec.region` (some ACK types), and that
    annotation (adopted ACK resources). `transform.derive` with `coalesce` takes the
    first present, so a panel does not branch on provider.
  - **A skew matrix must show the count, not just the colour.** A cell coloured for
    "holds several values" while displaying one of them shows a tag and implies others
    exist — the reader has to hover to learn that. Cells now render `tag +N`.
  - **Integer data must not get fractional buckets.** Auto-binning replica counts at
    0.2 produces `1.0–1.2`: an edge nothing can fall in.
- **M3 — pinned dimensions, compliance, and save-as-Filter. Built.** Value-recording
  Trigger creation, compliance panels that run validators server-side, and promoting a
  panel's query to a real ConfigHub Filter.

  What building it settled:

  - **Recording depends on *selection*, not existence.** A Space selects Triggers through
    `WhereTrigger` / `TriggerFilterID`, and by default selects only the Triggers defined
    in itself. The org tested had a `get-image` recording Trigger in `confighub-system`
    that produced nothing, because the only Space selecting it was `confighub-system` —
    which holds **0 units**. Nothing errors; it reads as a broken feature. The dialog
    therefore reports, per Trigger, how many Spaces select it and how many units those
    Spaces hold, and flags the dead case.
  - **So configboard does not attach Triggers itself.** Attaching means editing Spaces
    the app does not own, which would break the property that it writes only its own
    entities. It creates the Trigger and the Filter in its own Space and *generates* the
    `cub space update --patch … --trigger-filter … --where-trigger "-"` commands, plus the
    backfill patch — because values are recorded on the next mutation, so a new Trigger
    changes nothing until something touches the data.
  - **A wildcard `where_trigger` does not return.** `FunctionName LIKE 'vet-%'` runs every
    validator against every candidate Unit; it never completed on a 403-Unit org. One
    named validator returns in ~22 seconds — and scoping with `where` barely helps,
    because the validator sweep dominates, not the candidate count.
  - **Which makes auto-running wrong.** Compliance panels are `manual: true`: they state
    the cost and wait to be asked. Spending 20 seconds of someone's server time because a
    tab was opened is not a thing to do silently.
  - **Gates and warnings need no sweep.** `LEN(ApplyGates) > 0` and
    `LEN(ApplyWarnings) > 0` are already recorded on the Unit by whichever Trigger
    produced them, so those panels are ordinary metadata queries and run immediately.
    The `Finding` source explodes those maps into one row per failing check, keyed
    `<policy-space>/<trigger>/<function>` — so "which guardrail fires most, and is it
    live?" costs one unit query.
  - **Recorded findings are state, and a data revision is what recomputes them.** What
    was observed on a real org while wiring the manager guardrail packs:

    | action | re-lists a Space's Triggers | re-evaluates its Units |
    | --- | --- | --- |
    | attach a Filter to a Space with no prior selection | yes | yes |
    | `--refresh-triggers` after demoting / disabling / widening | yes | not yet |
    | any data revision (`UpdateUnit`, `set-annotation`, a function) | — | yes |

    The middle row is a **known server bug being fixed**, not a design property: a
    refresh is meant to re-evaluate, and while it doesn't, a Space can report its full
    Trigger set while carrying findings computed against the old set. One prod Space
    showed 20 Triggers selected and zero findings while an on-demand run of the same
    check found 10 failing Units; a `set-annotation` across those Units recomputed them
    and recorded matched on-demand exactly. Once the bug is fixed this row becomes "yes"
    and nothing in configboard needs to change.

    Two things worth keeping regardless of the fix. First, `set-annotation` recomputes
    because it writes into the **config data**, so a Mutation Trigger fires; a Unit-level
    `--label` patch touches ConfigHub entity metadata and does not — a useful distinction
    when you want to force a re-evaluation deliberately. Second, an on-demand run of the
    same check is a cheap way to confirm a zero is real, which is part of why the
    `manual: true` panels exist alongside the recorded ones.
  - **One `TriggerFilterID` per Space, so multi-pack adoption needs one combined
    Filter.** Five guardrail packs each ship their own Filter, and a Space can point at
    exactly one — so `Labels.Pack IN (…)` across packs is the only way to adopt more than
    one. Whether that Filter also carries `AND Warn = true` is load-bearing: with it, a
    Space's *blocking* baseline Triggers stop matching and it silently loses enforcement.
  - **`color: status` is only for categories that are states.** A by-Space bar chart with
    the status role renders every bar neutral, because a Space slug matches no state word
    — a chart with no magnitude at all.

Each milestone is independently demoable; M0 alone is a credible example.

## 10. Decisions and deferrals

**Settled**

- **Name:** `configboard`.
- **Scope:** one organization, one ConfigHub instance. Cross-org and
  multi-instance are not supported — not "later", not a hidden config knob.
- **Refresh:** manual refresh plus a 60s stale time. No websockets.
- **Units with no Target are not a gap in the data — they are a category.** An
  unbound Unit is typically a base unit held for cloning, or configuration that is
  not deployable on its own (an `AppConfig/YAML` unit consumed through a Link).
  So `TargetID IS NULL` is a *dimension*, not a defect: cluster-grain panels
  legitimately exclude those rows, and the panel frame states the exclusion and its
  count rather than silently narrowing. A "deployable vs base/undeployable" split
  is itself a useful composition panel.
- **Grain discipline:** Unit-grain panels join `Target`; Space-grain panels join
  `ReleaseTarget`. One chart never mixes the two.

**Deferred**

- **Apply _failure_ trends.** Revisions record what landed (`LiveAt`), so the
  delivery panels chart successful applies. A failed-apply rate over time is not
  derivable from Revisions alone; it is out of scope rather than approximated from
  unit state.
- **Trends over _state_.** Revisions carry timestamps, so trends over *changes* are
  easy; "how many units had >3 replicas each week" requires replaying revision data.
  Out of scope for M0–M3, and stated in the README so the gap reads as a decision.

**Resolved by measurement** (see §2)

- Joined `where` terms work, do not require `include`, and cost no more than a native
  predicate at this org size. The `SpaceID IN (…)` fallback is not needed.
- `!= ''` does not test presence; `IS NOT NULL` does.
- Space summary counts agree exactly with the equivalent unit-level `where`.

**Open**

- **Behaviour at a larger org size.** All of the above was measured at ~400 Units.
  The no-pagination limit on `GET /unit` is the thing that will bite first; the row
  budget (§7) states it rather than hiding it, but the real ceiling is unmeasured.
- **The cluster dimension is thin until Spaces are migrated.** `ReleaseTargetID` is
  newly implemented, so existing Spaces do not have it set yet: in the org tested, 274
  of 398 Units are bound to a Target while **no** Space has a `ReleaseTargetID`, and no
  Target has collected `Cluster.*` facts. Unit→`Target` therefore carries every
  cluster-grain panel today, and the Space-grain `ReleaseTarget` panels light up as
  Spaces adopt the field — nothing to design around, but `preflight.sh` reports all
  three counts so an empty panel is explained rather than mysterious.

## References

- [Filters and Views guide](https://docs.confighub.com/guide/filters-and-views/) —
  quick filters, value-recording Triggers, operational dashboard views
- [Filters concept](https://docs.confighub.com/background/concepts/filters/) —
  AND-only `where`, the `Data.` prefix
- [Attributes](https://docs.confighub.com/background/entities/attribute/) —
  custom getters/setters, defaults mode
- [Views](https://docs.confighub.com/background/entities/view/) — column types
- [Targets](https://docs.confighub.com/background/entities/target/) — `Facts`,
  including the collected `Cluster.*` keys
- OpenAPI spec — the authority on `where` / `include` / `select` / `summary`
  parameters and the includable fields per entity
- JS SDK — [github.com/confighub/js-sdk](https://github.com/confighub/js-sdk)
  (`@confighub/react-auth`, `@confighub/rtk-query`); bump `.spec-version` to `v0.2.0`
- Prior art in this repo — `promoter` (SPA on the JS SDK, config-as-data storage),
  `cost-estimator` / `sec-scanner` (analysis over config data)
