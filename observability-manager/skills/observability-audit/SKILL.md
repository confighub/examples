---
name: observability-audit
description: 'Inventory the observability posture of Kubernetes workloads stored in ConfigHub across a fleet with the cub-observability CLI — ServiceMonitors, Services (and which expose metrics), and telemetry-sidecar presence. Use for "what ServiceMonitors / Services do we have?", "which Services expose metrics?", "which workloads have an otel sidecar?", "per-cluster observability counts". Not for ServiceMonitor coverage gaps (use observability-findings), fixes (use observability-instrument / observability-guardrails), or live cluster state (use kubectl); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-observability --help) Bash(cub-observability * --help) Bash(cub auth status) Bash(cub-observability preflight) Bash(cub-observability snapshot) Bash(cub-observability snapshot *) Bash(cub-observability list) Bash(cub-observability list *) Bash(cub-observability coverage) Bash(cub-observability coverage *) Bash(cub-observability sidecars) Bash(cub-observability sidecars *)
---

# observability-audit

Inventory the observability resources ConfigHub holds across the fleet — ServiceMonitors, Services, and telemetry sidecars — and see which metrics-exposing Services are actually scraped. Read-only.

## Why this matters

Prometheus **ServiceMonitor coverage** is a cross-Unit property: the ServiceMonitor and the Service it selects live in separate Units, so a per-Unit validator can't tell whether a metrics Service is scraped. `cub-observability` loads a fleet snapshot and joins them. Analysis is over **ConfigHub-managed Units only**. A Service is considered metrics-exposing if it has a port named metrics/http-metrics/monitoring/prometheus/telemetry or a `prometheus.io/scrape=true` annotation. Output is JSON by default; add `-o table`.

## When to use

- "What ServiceMonitors / Services do we have?" → `snapshot`, `list`.
- "Which Services expose metrics, and are they scraped?" → `coverage` (`--uncovered-only` for gaps).
- "Which workloads have an otel sidecar?" → `sidecars` (`--missing-only`, `--sidecar`).
- "Per-cluster observability counts." → `snapshot`.

## Do not load for

- The ranked gap list — use **observability-findings**.
- Adding a ServiceMonitor or sidecar — use **observability-instrument**.
- Enforcement — use **observability-guardrails**.
- Live cluster state — `kubectl`.

## Preflight gates

1. `cub-observability preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

- `cub-observability snapshot -o table` — per-cluster ServiceMonitor / Service / metrics-Service / workload counts.
- `cub-observability coverage -o table` — per metrics-Service, the covering ServiceMonitor (or MISSING).
- `cub-observability sidecars -o table` — per workload, telemetry-sidecar presence.
- `cub-observability list -o table` — ServiceMonitor + Service explorer.

Scope server-side with `--where` + label shorthands (`--component`, `--environment`, …); `--cluster` / `--namespace` are client-side.

## Stop conditions

- Snapshot empty — report it; suggest widening scope or checking `cub auth status`.
- The question is the ranked gap list or a fix — hand off to **observability-findings** / **observability-instrument**.

## Tool boundary

Read-only. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-observability snapshot --help`, `… coverage --help`, `… sidecars --help`, `… list --help`.
- Companion skills: **observability-findings**, **observability-instrument**.
