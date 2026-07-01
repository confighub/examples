# cub-observability — Observability posture manager for ConfigHub

`cub-observability` manages the **observability posture of Kubernetes workloads**
as data in ConfigHub across a fleet: Prometheus **ServiceMonitor coverage** of
metrics-exposing Services, and **OpenTelemetry / telemetry sidecar** injection. It
is designed for use by an AI agent in a terminal, and is a sibling of
[`rbac-manager-for-agents`](../rbac-manager-for-agents),
[`network-policy-manager`](../network-policy-manager),
[`namespace-manager`](../namespace-manager),
[`workload-manager`](../workload-manager), and
[`scheduling-manager`](../scheduling-manager).

**The cross-Unit check.** Whether a metrics Service is actually scraped is a
property of *two* Units — the Service and the ServiceMonitor that selects it. A
per-Unit validator wired as a Trigger sees only one of them, so it can't tell.
`cub-observability` does the selector join over the whole fleet.

**The sidecar find-or-append.** `inject-sidecar` uses the `set-path` function to
find-or-append the collector container by name, so a re-run replaces it rather
than duplicating — the reason `set-path` was built.

## Tutorial

[TUTORIAL.md](TUTORIAL.md) walks the whole flow end to end with real commands and
output — audit coverage, author a ServiceMonitor, inject a sidecar, and enforce.

## Build

```
make build      # -> bin/cub-observability
```

Runs standalone (`bin/cub-observability ...`) or as a cub plugin
(`cub observability ...`). ConfigHub I/O uses your existing cub session — run
`cub auth login` first.

## Commands

Read commands default to JSON (`-o json`); pass `-o table`. Write commands are
**dry-run by default** and require `--commit --change-desc`; they edit Units but
never apply to a cluster (that's a separate `cub unit apply`), and never bypass
ApplyGates.

| Command | Kind | What it does |
|---|---|---|
| `preflight` / `version` | diag | Verify the ConfigHub session; print version |
| `snapshot` | read | Per-cluster ServiceMonitor / Service / metrics-Service / workload counts |
| `list` | read | Enumerate ServiceMonitors and Services across the fleet |
| `coverage` | read | Per metrics-Service, is a ServiceMonitor selecting it? (the cross-Unit join) |
| `sidecars` | read | Per workload, telemetry-sidecar presence |
| `findings` | read | Severity-ranked findings (uncovered metrics Service; dangling ServiceMonitor) |
| `ensure-servicemonitor` | write | Author a ServiceMonitor whose selector is derived from a Service |
| `inject-sidecar` | write | Inject/replace an otel sidecar via `set-path` (find-or-append by name) |
| `profile install\|list\|apply` | write | The profile library (parameterized Invocations, e.g. `otel-sidecar`) |
| `fleet-edit` | write | Apply a profile across a `--where` selector |
| `promote` | write | Override-preserving upgrade of downstream Units to upstream |
| `guardrails install\|status\|annotate` | write | Enforcement — `vet-cel` + annotate-then-validate for ServiceMonitor coverage |

## Agent skills

The `skills/` directory holds ConfigHub agent skills (a `SKILL.md` plus `evals/`):

| Skill | Kind | Covers |
|---|---|---|
| `observability-audit` | read | `snapshot`, `list`, `coverage`, `sidecars` |
| `observability-findings` | read | `findings` |
| `observability-instrument` | write | `ensure-servicemonitor`, `inject-sidecar`, `profile`, `fleet-edit`, `promote` |
| `observability-guardrails` | write | `guardrails install\|status\|annotate` |
