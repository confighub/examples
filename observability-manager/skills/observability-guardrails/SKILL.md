---
name: observability-guardrails
description: 'Install and operate the observability enforcement pack with the cub-observability CLI — an observability-policy Space with a Warn=true vet-cel Trigger that flags metrics-exposing Services with no ServiceMonitor, fed by annotate-then-validate (coverage is a cross-Unit property). guardrails install (dry-run by default), status (Units with ApplyWarnings/Gates), annotate (write the coverage finding onto uncovered Service Units). Use for "enforce that metrics services are scraped", "warn on services with no ServiceMonitor", "which services are flagged?". Not for read-only findings (use observability-findings) or fixing coverage (use observability-instrument).'
phase: act
allowed-tools: Bash(cub-observability --help) Bash(cub-observability * --help) Bash(cub auth status) Bash(cub-observability preflight) Bash(cub-observability guardrails) Bash(cub-observability guardrails *)
---

# observability-guardrails

Turn the ServiceMonitor-coverage gap into **enforcement** — an advisory ApplyWarning (promotable to a blocking ApplyGate) that fires in the normal apply pipeline.

- **`guardrails install`** — creates the `observability-policy` Space, a `Warn=true` `vet-cel` Trigger (`servicemonitor-coverage`), and a shared Trigger Filter, then wires in-scope Spaces to it. **Dry-run by default**; re-run with `--commit`.
- **`guardrails status`** — lists Units carrying observability ApplyWarnings or ApplyGates.
- **`guardrails annotate`** — writes the `observability.confighub.com/coverage` finding onto each uncovered metrics-Service Unit (the producing half of annotate-then-validate). Dry-run unless `--commit --change-desc`.

## Why this matters

ServiceMonitor coverage is a **cross-Unit** property (the ServiceMonitor and the Service are separate Units), so a per-Unit `vet-cel` can't compute it directly. This uses **annotate-then-validate**: `annotate` computes coverage and stamps the finding on the uncovered Service Unit, and the Trigger turns that annotation into a warning. `install` is conservative — it skips Spaces that already select Triggers their own way rather than clobbering them.

## When to use

- "Enforce that metrics Services are scraped." → `guardrails install` (preview), then `--commit`, then `guardrails annotate --commit`.
- "Warn on Services with no ServiceMonitor." → the `install`ed Trigger + `annotate`.
- "Which Services are flagged?" → `guardrails status`.

## Do not load for

- Read-only findings — use **observability-findings**.
- Fixing coverage — use **observability-instrument**.
- General Trigger/ApplyGate mechanics beyond this pack — use `triggers-and-applygates`.

## Preflight gates

1. `cub-observability preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. The user has permission to create the policy Space, Trigger, and Filter, and to set Spaces' TriggerFilterID.

## The loop

1. **Preview** (dry-run — the default): `cub-observability guardrails install -o table`.
2. **Install** with `--commit`. The Trigger is `Warn=true` (advisory); promote to blocking with `cub trigger update servicemonitor-coverage --space observability-policy --unwarn`.
3. **Feed the finding**: `cub-observability guardrails annotate` (dry-run), then `--commit --change-desc`. Re-run after adding ServiceMonitors so the cached annotation stays fresh.
4. **Check**: `cub-observability guardrails status -o table`.
5. **Fix** flagged Services via **observability-instrument**, then re-check.

## Stop conditions

- A Space is skipped (custom WhereTrigger / different TriggerFilterID) — report it; wiring means adding the guardrail Filter to that Space's existing selector, not overwriting.
- The user wants a blocking gate now — install advisory first, verify, then `--unwarn`.

## Tool boundary

Allowed: `guardrails install` (dry-run unless `--commit`), `status`, `annotate` (dry-run unless `--commit --change-desc`). Not allowed: bypassing gates, `kubectl` mutations, applying to clusters. Fixing coverage is **observability-instrument**.

## References

- `cub-observability guardrails --help`, `… install --help`, `… status --help`, `… annotate --help`.
- Companion skills: **observability-instrument**, **observability-findings**, `triggers-and-applygates`.
