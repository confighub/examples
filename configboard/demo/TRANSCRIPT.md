# configboard demo — transcript

Companion text for `configboard-demo.mp4` (3 min 11 s, silent, captions burned in).
The video has no audio: the captions on screen are the narration, and this file is
the longer form of the same walkthrough.

Everything shown is live data from a real ConfigHub organization — 85 Units across
24 Spaces, four clusters, six bundled dashboards. Nothing is mocked, and no numbers
were edited after the fact.

---

## 1 — What it is

**0:00 · Fleet Overview**
> configboard — BI-style dashboards over ConfigHub configuration data. Fleet Overview:
> 85 Units under management, 25% applied and current, nothing blocked by an apply gate.

configboard is a small React app that treats ConfigHub as a BI backend. Selection,
projection, and joins happen on the server through the same `where` clauses the CLI
uses; grouping, binning, and aggregation happen in the browser, because ConfigHub
deliberately has no `GROUP BY`. Four KPI tiles and four charts here, all derived from
one `GET /space?summary=true` and one `GET /unit`.

**0:06 · Show me the query**
> Every panel can show its query. The code icon reveals the equivalent cub command —
> here `cub space list`, which uses the one server-side rollup ConfigHub offers.

No panel is a black box. The `<>` icon on every panel prints the `cub` command that
produces the same rows, so a number you distrust is one copy-paste away from being
checked in a terminal. That constraint also kept the app honest while it was being
built: if a panel could not be expressed as a `cub` command, it was doing something
the API does not actually support.

**0:12 · Cross-filtering**
> Clicking a bar cross-filters the whole dashboard. A chip records the scope, and
> Clear all removes it.

Clicking a mark narrows every other panel on the page. The scope is always visible as
a chip rather than implied, so you can never be looking at a filtered dashboard without
knowing it.

---

## 2 — Compliance, from data that already exists

**0:17 · Compliance**
> Compliance: 36 open findings across 17 Units. These come from the guardrail Triggers
> the fleet managers already install — configboard reads recorded findings rather than
> running its own scan.

This dashboard is the one that changed most during the build. The first version invoked
validators itself, which was slow and found little. The rewrite reads `ApplyGates` and
`ApplyWarnings` off the Units — fields the platform already maintains — so compliance
became a metadata query with no extra work at query time. The guardrail packs installed
by the fleet managers (rbac, network-policy, namespace, workload) are what populate them.

**0:24 · Findings by check**
> Findings by check. One row per failing check, so a Unit failing three guardrails
> counts three times.

A `Finding` in configboard is a row per *failing check*, not per Unit. That is the only
grain at which "which guardrail fires most" is answerable. The key each finding carries
is `<policy-space>/<trigger>/<function>`, split into three separate dimensions so you can
group by the policy Space, the individual check, or the validator behind it.

**0:29 · Check × Space**
> Check × Space heatmap — where each guardrail is actually failing.
> workload-termination-message-policy dominates, and apptique-prod carries most of it.

The heatmap is the pivot: one glance tells you whether a failure is a fleet-wide policy
problem or one team's backlog.

**0:35 · On-demand panels**
> A panel marked `manual: true` never runs on load. This one invokes vet-placeholders
> across the fleet only when asked — expensive queries stay opt-in.

Some questions are worth about twenty seconds of server time. Those panels are marked
`manual: true` in the document, render a button instead of a chart, and say what they are
about to cost before you press it.

---

## 3 — Reading values out of configuration

**0:41 · Version Skew**
> Version Skew reads values out of the configuration data through a saved View, so it
> works for resource types the app has never heard of — Crossplane, ACK, anything.

This is the part that generalizes. A ConfigHub View projects a data path out of a Unit's
configuration as a column — `spec.template.spec.containers.0.image`, or an annotation on
a Crossplane managed resource. configboard groups and charts those columns without knowing
what the resource type is, which is why the same dashboard covers Kubernetes workloads,
Crossplane MRs, and ACK resources.

**0:47 · Variables from live data**
> Dashboard variables are populated from live data. These Environment values come from
> Space labels.

**0:53 · Applying a variable**
> Choosing prod rewrites every panel's where clause and re-queries server-side.

Variable substitution happens inside the `where` clause, so narrowing to prod is a
smaller query, not a client-side filter over the same rows.

**0:58 · Image tag × environment**
> Image tag × environment. Read across a row: that is where an environment being behind
> shows up. A coloured cell holding several tags is labelled rather than silently collapsed.

A heatmap cell that holds more than one distinct value shows the dominant one plus a
`+N` count, and the legend says so. Collapsing several tags into one cell without saying
so would be the lie that makes this kind of chart untrustworthy.

**1:04 · Honest empty states**
> This org runs no Crossplane or ACK resources, so those panels say so —
> "85 rows had no value for this dimension" instead of an empty chart that looks broken.

Two panels here have nothing to show, and they say why. An empty chart is indistinguishable
from a broken one; a sentence naming the row count fixes that.

---

## 4 — Counting at resource grain

**1:10 · Resource Inventory**
> Resource Inventory counts at resource grain, not Unit grain: 154 resources, 16 distinct
> types, 5 namespaces — inside the same 85 Units. The counts come from the get-resources
> function.

85 Units contain 154 resources. Every other dashboard counts Units, because that is what
the list API returns; this one calls the `get-resources` function with `body=none` to get
resource metadata without pulling any config bodies, and counts what is actually inside.

**1:16 · Resources by kind**
> Resources by kind: 38 Services, 37 Deployments, 30 NetworkPolicies. The fleet's real shape.

**1:22 · Kind by provider family**
> Kind by provider family separates Kubernetes core objects from installer CRDs.

**1:27 · Table view**
> Every chart has a table view — the same frame, read exactly, for accessibility and for
> copying.

Every panel toggles to the exact numbers behind it. This is an accessibility requirement,
not a nicety: colour and length are approximations, and some readers need the value.

---

## 5 — Dashboards are configuration

**1:32 · The source editor**
> A dashboard is a Unit. The source editor edits its YAML document in place and saves the
> result as a new ConfigHub revision.

A configboard dashboard is an `AppConfig/YAML` Unit in a `configboard` Space. That is the
whole storage design: dashboards get revision history, diffs, review, promotion between
environments, and access control for free, because they are ordinary ConfigHub configuration.

**1:38 · The panel builder**
> Add a panel: choose a source, a grouping, a second grouping for series, a measure, and a
> chart form.

**1:43 · Live preview**
> The preview renders live against real data while you build, so a bad grouping is visible
> immediately.

**1:49 · The stanza it will write**
> It also shows the YAML stanza it will append. The builder teaches the document format
> rather than hiding it, so the next panel can be hand-written.

The builder is a teaching tool as much as an editor. It shows the exact YAML it is about
to append, which is how someone graduates from clicking to writing the document directly.

---

## 6 — Creating a new dashboard

**1:55 · Duplicate**
> Creating a new dashboard: duplicate an existing one, then give it a title and a slug.

The title is free text — it is stored in a ConfigHub Annotation, which accepts prose. The
slug identifies the Unit and follows ConfigHub's stricter naming rules. Two fields,
two different character sets, for a real reason.

**2:00 · The new dashboard**
> The new dashboard appears as its own tab, backed by a new Unit in the configboard Space.

**2:06 · Building its first new panel**
> Building a panel on the new dashboard: source Resource, grouped by namespace, series by
> kind, stacked bar.

**2:11 · Saved and rendering**
> Saved, and rendering — workloads per namespace, stacked by kind.

**2:17 · The save is a revision**
> The save is a ConfigHub revision. The dashboard is now at revision 2, so a dashboard's
> history is just a Unit's history — diffable and revertible.

This closes the loop. The panel added through the UI is the same YAML you would have typed,
stored in the same Unit, at revision 2. `cub revision list` shows the dashboard's history;
`cub unit update --restore 1` undoes the panel.

---

## 7 — Making a value filterable

**2:23 · Pinned dimensions**
> Pinned dimensions. A View's data-path dimension can be grouped and charted but never
> filtered server-side. Recording that value into Unit.Values makes it indexed metadata a
> where clause can use.

The one real limit in the View approach: a projected data path can be grouped, but it
cannot appear in a `where` clause, and every query reparses configuration to read it. The
fix is a Mutation Trigger that records the value into `Unit.Values`, where it becomes
indexed metadata — `Values."Image/container-image"` then works in `where` and arrives with
the plain unit list.

**2:30 · Generated, not executed**
> Attaching a recording Trigger means editing Spaces this app does not own, so configboard
> generates the cub commands instead of running them.

Recording depends on *selection*, not existence: a Space selects Triggers via its
`WhereTrigger` and `TriggerFilterID`, and by default selects only Triggers defined in
itself. So a recording Trigger has to be attached to the Spaces holding your workloads —
Spaces this app has no business editing. It prints the commands, with the reasoning, and
leaves running them to you.

---

## 8 — Delivery and posture

**2:37 · Delivery Health**
> Delivery Health: changes per day by environment, read from Revision history.

Revisions are the event stream ConfigHub already keeps, so change frequency, what makes
changes (`Revision.Source`), and churn per Unit need no additional instrumentation.

**2:42 · Reported exclusions**
> Lead time reports its own exclusions — 51 revisions never went live, so they cannot have
> one. An honest empty panel beats a misleading number.

Lead time is measured from a revision being created to it going live. Revisions that never
went live have no lead time, and the panel says how many it dropped rather than quietly
computing a median over a biased subset.

**2:48 · Fleet Posture**
> Fleet Posture: Kubernetes versions across clusters, read from Target facts.

Cluster version comes from `Target.Facts["Cluster.KubernetesVersion"]` — reported by the
worker, so version skew across clusters needs no cluster access from the browser.

**2:54 · Server-side rollups**
> Gated, unapproved, and unapplied counts per Space, from the server-side rollup.

`GET /space?summary=true` is the only aggregation ConfigHub does server-side. Where it
answers the question, configboard uses it instead of counting Units in the browser.

---

## 9 — Close

**2:59 · Deleting a dashboard**
> Deleting a dashboard deletes a Unit in the configboard Space. The configuration it charts
> is untouched.

**3:05 · Back to six**
> Back to the six bundled dashboards. Everything shown here is a YAML document you can
> edit, review, version, and promote like any other ConfigHub configuration.

---

## How the recording was made

No screen recorder and no video tooling were installed. Frames were captured from the
running app through Chrome DevTools, then encoded with `encode.swift` — a standalone
AVFoundation script that burns the captions in and writes a silent H.264 `.mp4`:

```
swift encode.swift manifest.tsv configboard-demo.mp4
```

`manifest.tsv` holds one line per frame: PNG filename, seconds on screen, caption.
To re-record, recapture the frames, edit the captions, and re-run. Capture starts after
sign-in, so no account or organization identity appears in any frame.
