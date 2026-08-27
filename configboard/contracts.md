# Contracts: configboard

Stable, machine-checkable behavior for this example. See
[EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for the format.
configboard is a read-only webapp, so these contracts cover its preflight output, the
dashboard document schema, and the API requests it issues.

### `./preflight.sh --json`

- mutates: no
- mutates_confighub: no
- mutates_live_infra: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `counts.{spaces,units,targets}`,
  `dimensions.{Component,Environment,Region}.{spaces,values}`,
  `cluster_dimension.{units_with_target,spaces_with_release_target,targets_with_cluster_facts}`
- proves: whether this organization has the dimensions the dashboards slice by, before
  anyone opens an empty panel
- exit code: 0 on success; 1 if `cub`/`jq` is missing or `cub` is not authenticated

### `./preflight.sh`

- mutates: no
- output shape: plain text
- stable text anchors: `configboard preflight (read-only)`, `Space labels (what dashboards slice by)`,
  `Cluster dimension (a Target reference, not a label)`
- proves: the same, human-readable, plus the exact `cub` commands to fill each gap

### Storage Space

- slug: `configboard`
- created **only** on the first write (seed / duplicate / save); reading never creates it
- labels: `app=configboard`
- mutates live infra: no

### Dashboard unit

- one Unit per dashboard in the `configboard` Space
- `ToolchainType`: `AppConfig/YAML`
- labels: `app=configboard`
- `Data`: the dashboard document (YAML), stored verbatim including comments
- `DisplayName`: a sanitized form of the document `title` — omitted when the title
  contains nothing matching `^[A-Za-z0-9]([\-_ .|A-Za-z0-9]*[A-Za-z0-9.!?])?$`. The
  document `title` is authoritative for display; DisplayName is a convenience label.
- every save is a merge-patch producing a new revision with a `LastChangeDescription`
- discoverable via: `cub unit list --space configboard --where "Labels.app = 'configboard'"`

### Recording Triggers and Filters (M3)

- a value-recording Trigger is created **in the `configboard` Space only**, on request:
  `Event = Mutation`, `ToolchainType = Kubernetes/YAML`, a readonly function
  (`get-image`, `get-replicas`), producing `Unit.Values["<trigger-slug>/<attribute>"]`
- `cb-recorded-values` Filter (`From = Trigger`) is created alongside it, so the generated
  `--trigger-filter` reference resolves
- configboard **never** attaches a Trigger to a Space it does not own. It generates:
  `cub space update --patch <space> --trigger-filter configboard/cb-recorded-values --where-trigger "-"`,
  the backfill patch, and the verification query
- recording requires **selection**: a Space selects Triggers via `WhereTrigger` /
  `TriggerFilterID`, and by default selects only its own. A Trigger no unit-holding Space
  selects records nothing, silently
- values are recorded on the **next** mutation of each Unit; existing Units need a no-op
  patch to backfill
- "save as Filter" writes a Filter into the `configboard` Space named `cb-<panel-id>`,
  usable as `cub unit list --filter configboard/cb-<panel-id>`

### Seeded Views

- created **only** when the user asks, in the `configboard` Space
- `cb-workload`: `Unit`, `Component`, `Environment` (metadata) + `Image`, `Replicas`
  (data paths) for Kubernetes workloads
- `cb-cloud-resource`: region and size across Crossplane and ACK, including the ACK
  annotation path `metadata.annotations.services~1k8s~1aws/region` (a `.` inside a map
  key escapes as `~1`)
- reusable outside the app: `cub unit list --view configboard/cb-workload`
- a panel references a View as `space/slug`; the app resolves it to a UUID because the
  org-wide unit list accepts only a UUID for `view`
- a view query always passes `include=SpaceID,TargetID`: a View's metadata columns
  return **null**, silently, without the related entity included

### Dashboard document

- location: `dashboards/*.yaml` bundled as seed material; the stored Units are the
  source of truth once seeded
- `apiVersion`: `configboard.confighub.com/v1`
- `kind`: `Dashboard`
- required: `slug` (lowercase, `[a-z0-9-]+`), `title`, `panels` (non-empty)
- `variables[]`: `{ name, label, type?: select|timeRange, from?: {spaceLabel|target}, default?, allValue? }`
- `panels[]`: `{ id, title, description?, span?, query, transform?, chart }`
- `query`: `{ source: Unit|Space|Revision|Target|Resource, where?, view?, filter?, resourceType?, triggerWhere?, triggersPassed?, manual?, excludes? }`
- `query.triggerWhere`: selects Triggers to run as part of the query; name **one**
  validator (`FunctionName = 'vet-placeholders'`). A pattern runs every validator against
  every candidate Unit and does not return.
- `query.triggersPassed`: keep passing Units instead of failing ones (default: failing)
- `query.manual`: the panel does not fetch until the user clicks Run — for queries whose
  cost is seconds of server time
- `transform`: `{ derive?, bin?, numericBin?, groupBy?, aggregate?, topN?, tail?, sort?, dropEmpty? }`
- `transform.derive`: `[{ name: 'Derived.X', coalesce: [dimA, dimB] }]` — first non-empty
  source wins; derived names must start with `Derived.` and are never pushed into `where`
- `transform.numericBin`: `{ field, size? }` — histogram buckets; auto-sizing never
  gives integer data a fractional width
- `aggregate.fn`: `count | sum | avg | min | max | p50 | p95 | distinctCount | value`
  (`value` measures how many distinct values a group holds and labels the point with the
  dominant one — what a skew matrix needs)
- `transform.tail`: `other` (default, fold the residue into an "Other" bucket) or
  `drop` (omit it and report the count) — `drop` for ranked top-N charts, where a folded
  residue would be the largest mark
- `chart.form`: one of `statTile | meter | bar | stackedBar | line | donut | heatmap | histogram | table`
- validated by: `cd app && npm test` — an unknown dimension, an unknown chart form, an
  undeclared variable reference, or a `meter` without `chart.totalField` fails the suite

### Variable substitution

- `${var}` is replaced with the variable's current value
- `${var.start}` resolves a `timeRange` variable to an absolute ISO timestamp
- a variable set to **All** (`*`) or left unset **drops its entire conjunct** from the
  `where` expression — it never matches the literal string
- conjunct splitting is quote-aware: ` AND ` inside a single-quoted literal is not a
  separator
- proves: an unscoped dashboard issues an unfiltered query

### API requests issued

- charting is read-only: `GET /unit`, `GET /space`, `GET /revision`, `GET /target`, and
  `POST /function/invoke` with the non-mutating `get-resources` function
- writes happen only on explicit dashboard actions: `POST /space` (seed, first write
  only), `POST /space/{id}/unit` (seed, duplicate), `PATCH /space/{id}/unit/{id}` (save),
  `DELETE /space/{id}/unit/{id}` (delete)
- cross-filters are applied client-side and never change the request
- resource panels invoke `get-resources` with `body=none` (metadata only, never the
  resource bodies) and read `Outputs.ResourceList`
- unit queries always pass `select` and never request `Data`, `LiveData`, or
  `LiveState`
- space queries pass `summary=true`
- a panel naming a joined dimension also passes the matching `include`
  (`SpaceID`, `TargetID`, `UnitID`)
- panels compiling to an identical request share one fetch (RTK Query cache key)
- stale time: 60s; refresh is manual
- row budget: soft warning above 5,000 rows, refusal above 25,000, both stated in the
  panel footer

### Verified API behavior

Measured against a 56-Space / 398-Unit organization. These are the assumptions the
query layer rests on:

- `GET /space?summary=true` returns per-Space `TotalUnitCount`, `UnreleasedUnitCount`,
  `UnapprovedUnitCount`, `UnlinkedUnitCount`, `GatedUnitCount`, `WarnedUnitCount`,
  `UpgradableUnitCount`
- summed `UnreleasedUnitCount` equals the count from
  `where=HeadRevisionNum > LastReleasedRevisionNum`; summed `GatedUnitCount` equals
  `where=LEN(ApplyGates) > 0`
- `where=Space.Labels.<K> = '<v>'` and `where=Target.<field> = '<v>'` filter correctly
  on `GET /unit`, with or without the matching `include`
- `where=<field> IS NOT NULL` tests presence; `where=<field> != ''` does **not** —
  it matches rows where the field is absent
- `where_trigger=FunctionName = '<one-validator>'` with `triggers_passed=false` returns
  the failing Units in ~22s on a 398-Unit org; narrowing with `where` does not
  meaningfully reduce that, because the validator sweep dominates
- `where_trigger=FunctionName LIKE 'vet-%'` did not return at all
- `LEN(ApplyGates) > 0` and `LEN(ApplyWarnings) > 0` need no validator run: those are
  recorded on the Unit already, keyed `<policy-space>/<trigger>/<function>`
- recorded findings are **state**, recomputed by a data revision. Attaching a Filter to a
  Space that had no selection re-evaluates its Units; so does any data revision
  (`UpdateUnit`, `set-annotation`, a function). At the time of writing
  `--refresh-triggers` re-lists a Space's Triggers without re-evaluating — a known server
  bug being fixed — so after demoting, disabling, or widening, findings can lag until
  something writes data. An on-demand run of the same check confirms whether a zero is
  current.
