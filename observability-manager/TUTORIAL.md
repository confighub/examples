# Tutorial: managing observability posture with `cub-observability`

This walks `cub-observability` end to end against a live ConfigHub fleet: audit
ServiceMonitor coverage, author a ServiceMonitor, inject an OpenTelemetry sidecar,
and stand up enforcement. Every command and block of output below is real.

`cub-observability` reads and writes ConfigHub Units — it never touches a cluster.
Its writes create new Unit revisions; rolling those out is a separate `cub unit
apply`. Read commands default to JSON; add `-o table` for the human view used
here.

## Prerequisites

- A ConfigHub session — `cub auth login`.
- The binary: `make build` produces `bin/cub-observability`. It also runs as a
  `cub` plugin (`cub observability …`); this tutorial uses the
  `cub-observability` form.

## 1. Verify the session

```console
$ cub-observability preflight
cub-observability: ready (ConfigHub session valid)
```

## 2. See the fleet

`snapshot` is the per-cluster inventory — ServiceMonitors, Services (and how many
expose metrics), and workloads:

```console
$ cub-observability snapshot -o table
...
5 clusters, 0 ServiceMonitors, 38 Services (0 expose metrics), 41 workloads, 92 units (0 gated, 60 unapplied)
```

`coverage` reports, for each **metrics-exposing** Service (a port named
metrics/http-metrics/monitoring/prometheus/telemetry, or a `prometheus.io/scrape=true`
annotation), whether a ServiceMonitor selects it — the cross-Unit join a per-Unit
validator can't do. `findings` ranks the gaps (uncovered metrics Service = medium;
dangling ServiceMonitor = low).

## 3. Author a ServiceMonitor from a Service

`ensure-servicemonitor` reads a Service Unit's labels, namespace, and metrics port
and authors a matching ServiceMonitor Unit. Dry-run shows exactly what it would
create:

```console
$ cub-observability ensure-servicemonitor apptique-dev/frontend --port metrics
{
  "action": "create-servicemonitor",
  "dryRun": true,
  "space": "apptique-dev",
  "namespace": "apptique",
  "unit": "frontend-sm",
  "manifest": "apiVersion: monitoring.coreos.com/v1\nkind: ServiceMonitor\nmetadata:\n  name: frontend-sm\n  namespace: apptique\nspec:\n  selector:\n    matchLabels:\n      app: frontend\n  endpoints:\n  - port: metrics\n"
}
```

The selector (`app: frontend`) is derived from the Service's own labels, so it
matches by construction. Re-run with `--commit --change-desc "…"` to create the
Unit (it isn't applied to a cluster until you apply it). `ensure-servicemonitor`
refuses when the Service has no labels or no metrics port and no `--port` — it
won't guess.

## 4. Inject an OpenTelemetry sidecar

`inject-sidecar` adds a collector container to a workload's pod template using the
`set-path` function to **find-or-append** the container by name. First, `frontend`
has no sidecar:

```console
$ cub-observability sidecars -o table --cluster dev-cluster --namespace apptique
CLUSTER      NAMESPACE  KIND        NAME      SIDECAR
dev-cluster  apptique   Deployment  frontend  -
```

Inject it (dry-run by default; committing here):

```console
$ cub-observability inject-sidecar apptique-dev/frontend --image otel/opentelemetry-collector:0.100 \
    --commit --change-desc "Inject otel sidecar"
inject-sidecar: 1 of 1 Unit(s) changed
```

Now the sidecar is present — and because `set-path` finds-or-appends by container
name, re-running replaces it rather than adding a duplicate:

```console
$ cub-observability sidecars -o table --cluster dev-cluster --namespace apptique
CLUSTER      NAMESPACE  KIND        NAME      SIDECAR
dev-cluster  apptique   Deployment  frontend  otel-collector
```

## 5. Fleet-wide, with a profile

Profiles are named edits stored as ConfigHub Invocations. Seed the library once,
then list it:

```console
$ cub-observability profile install
Space observability-profiles ready
Profile observability-profiles/otel-sidecar ready

$ cub-observability profile list -o table
PROFILE       FUNCTION  PARAMS  DESCRIPTION
otel-sidecar  set-path  image   set-path: inject/replace an otel-collector sidecar container (param: image)
```

`fleet-edit` applies a profile across a `--where` selector in one operation
(dry-run first to see the blast radius):

```console
$ cub-observability fleet-edit --profile otel-sidecar --environment prod \
    --param image=otel/opentelemetry-collector:0.100
fleet-edit otel-sidecar: N of N Unit(s) would change (dry-run — pass --commit --change-desc to write)
```

`promote --component <c>` then carries an instrumentation change from a base Space
to its downstream variants (override-preserving).

## 6. Enforce with guardrails

ServiceMonitor coverage is a cross-Unit property, so a plain `vet-cel` on a
Service can't see whether a ServiceMonitor exists — it uses **annotate-then-validate**.
`guardrails install` stands up the `Warn=true` `vet-cel` rule and wires in-scope
Spaces; `guardrails annotate` writes the coverage finding onto uncovered Service
Units for the rule to read:

```console
$ cub-observability guardrails install -o table          # preview
$ cub-observability guardrails install --commit          # apply the pack
$ cub-observability guardrails annotate --commit --change-desc "Annotate uncovered services"
$ cub-observability guardrails status -o table           # Units now carrying warnings
```

Promote the rule to blocking later with
`cub trigger update servicemonitor-coverage --space observability-policy --unwarn`.

## Reference

- **Scoping** any command: `--where "<expr>"` (Unit / `Space.*` / `Target.*`,
  flat `AND`-only) plus shorthands `--component` / `--environment` / `--region` /
  `--owner` / `--layer` / `--variant`. `--cluster` / `--namespace` are client-side
  display filters.
- **Discipline**: reads default to JSON (`-o table` for humans); writes are
  dry-run until `--commit --change-desc`; nothing is applied to a cluster (roll
  out with `cub unit apply` separately); ApplyGates are never bypassed.
