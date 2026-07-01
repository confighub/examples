---
name: observability-instrument
description: 'Instrument Kubernetes workloads for observability as config-as-data with the cub-observability CLI — author a Prometheus ServiceMonitor for a Service (ensure-servicemonitor), inject an OpenTelemetry/telemetry sidecar container (inject-sidecar via set-path), one workload or across a selector via profiles. Use for "add a ServiceMonitor for the checkout service", "scrape this service", "inject an otel sidecar into web", "add the otel collector to every prod workload", "roll instrumentation downstream". Dry-run by default; requires --commit --change-desc. Not for read-only checks (use observability-audit / observability-findings), enforcement Triggers (use observability-guardrails), or applying to a cluster.'
phase: act
allowed-tools: Bash(cub-observability --help) Bash(cub-observability * --help) Bash(cub auth status) Bash(cub-observability preflight) Bash(cub-observability coverage *) Bash(cub-observability sidecars *) Bash(cub-observability findings *) Bash(cub-observability ensure-servicemonitor *) Bash(cub-observability inject-sidecar *) Bash(cub-observability profile) Bash(cub-observability profile *) Bash(cub-observability fleet-edit *) Bash(cub-observability promote *)
---

# observability-instrument

Instrument workloads — as data, dry-run by default. One workload, across a selector via a profile, or promoted downstream.

- **`ensure-servicemonitor <space>/<service-unit>`** — author a ServiceMonitor Unit whose selector is derived from the Service's labels (fixes uncovered metrics Services).
- **`inject-sidecar <space>/<unit> --image <img>`** — inject/replace an otel-collector sidecar via `set-path` (find-or-append the container by name).
- **`profile install | list | apply`** — the profile library (e.g. `otel-sidecar`, a parameterized `set-path` Invocation, `--param image=…`).
- **`fleet-edit --profile <slug> [--where …]`** — apply a profile across a selector in one operation.
- **`promote`** — override-preserving upgrade of downstream Units.

All **edit/create Units but do not apply them** to a cluster — rolling out is a separate `cub unit apply`.

## Why this matters

ServiceMonitor coverage is a cross-Unit property, and a sidecar must be find-or-appended into a pod template by name — `cub-observability` does both as data: `ensure-servicemonitor` authors a new Unit derived from the Service; `inject-sidecar` uses `set-path`'s associative find-or-append so a re-run replaces rather than duplicates. Everything is **dry-run by default** and requires `--commit --change-desc`; ApplyGates are never bypassed.

## When to use

- "Add a ServiceMonitor for the checkout service." → `ensure-servicemonitor <space>/checkout`.
- "Inject an otel sidecar into web." → `inject-sidecar <space>/web --image otel/opentelemetry-collector:0.100` (or `profile apply otel-sidecar … --param image=…`).
- "Add the otel collector to every prod workload." → `fleet-edit --profile otel-sidecar --environment prod --param image=…`.
- "Roll instrumentation downstream." → `promote --component <c>`.

## Do not load for

- Read-only checks — use **observability-audit** / **observability-findings**.
- Enforcement Triggers — use **observability-guardrails**.
- Applying Units to a cluster — that is `cub unit apply`.

## Preflight gates

1. `cub-observability preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. For `profile apply` / `fleet-edit`, the library exists — run `cub-observability profile install` once if `profile list` is empty.
3. The user has write permission on the target Space(s).

## The loop

1. **See the gap**: `cub-observability findings` / `coverage --uncovered-only` / `sidecars --missing-only`.
2. **Preview** (dry-run — the default):
   ```bash
   cub-observability ensure-servicemonitor apptique-dev/frontend
   cub-observability inject-sidecar apptique-dev/frontend --image otel/opentelemetry-collector:0.100
   ```
3. **Commit** with `--commit --change-desc` (summary + verbatim user prompt).
4. **Verify**: `cub-observability coverage --namespace <ns>` / `sidecars --namespace <ns>`.
5. **Roll out** is a separate step — hand off to `cub-apply`.

## Stop conditions

- `ensure-servicemonitor` refuses (Service has no labels, or no metrics port and no `--port`) — supply `--port`, or fix the Service.
- An ApplyGate attaches. **Do not bypass** — fix via **triggers-and-applygates**.
- The user wants to apply to a cluster — hand off to `cub-apply`.

## Tool boundary

Allowed: `ensure-servicemonitor`, `inject-sidecar`, `profile`, `fleet-edit`, `promote` (dry-run by default; `--commit` passes `--change-desc`), and the read commands. Not allowed: bypassing gates, `kubectl` mutations, applying to clusters.

## References

- `cub-observability ensure-servicemonitor --help`, `… inject-sidecar --help`, `… profile --help`, `… fleet-edit --help`, `… promote --help`.
- Companion skills: **observability-audit**, **observability-findings**, `kubernetes-resources`, `triggers-and-applygates`, `cub-apply`.
