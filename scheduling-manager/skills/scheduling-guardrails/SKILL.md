---
name: scheduling-guardrails
description: 'Install and inspect the placement enforcement pack with the cub-scheduling CLI — a scheduling-policy Space with a Warn=true vet-cel Trigger that flags controllers which tolerate a taint but do not constrain where they land (no nodeSelector or required node affinity), wired to in-scope Spaces via a shared Trigger Filter. guardrails install (dry-run by default) and status (Units with ApplyWarnings/Gates). Use for "enforce that tolerations come with placement", "warn on workloads that tolerate a taint but pin nothing", "which workloads are flagged?". Not for read-only findings (use scheduling-findings) or fixing placement (use scheduling-place).'
phase: act
allowed-tools: Bash(cub-scheduling --help) Bash(cub-scheduling * --help) Bash(cub auth status) Bash(cub-scheduling preflight) Bash(cub-scheduling guardrails) Bash(cub-scheduling guardrails *)
---

# scheduling-guardrails

Turn the placement anti-pattern into **enforcement** — an advisory ApplyWarning (promotable to a blocking ApplyGate) that fires in the normal apply pipeline.

- **`guardrails install`** — creates the `scheduling-policy` Space, a `Warn=true` `vet-cel` Trigger (`workload-toleration-needs-placement`), and a shared Trigger Filter, then wires in-scope Spaces to it. **Dry-run by default**; re-run with `--commit`.
- **`guardrails status`** — lists Units carrying placement ApplyWarnings or ApplyGates.

## Why this matters

The rule is a plain **per-resource `vet-cel`** check — a single Unit answers it (does this controller have tolerations but no nodeSelector and no required node affinity?), so no annotate-then-validate is needed. The manager can't set warnings directly; only a failed Trigger can. `install` is conservative: it skips Spaces that already select Triggers their own way (a custom WhereTrigger or a different TriggerFilterID) rather than clobbering them.

## When to use

- "Enforce that tolerations come with real placement." → `guardrails install` (preview), then `--commit`.
- "Warn on workloads that tolerate a taint but pin nothing." → the installed `vet-cel` Trigger.
- "Which workloads are flagged?" → `guardrails status`.

## Do not load for

- Read-only findings — use **scheduling-findings**.
- Fixing placement — use **scheduling-place**.
- General Trigger/ApplyGate mechanics beyond this pack — use `triggers-and-applygates`.

## Preflight gates

1. `cub-scheduling preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. The user has permission to create the policy Space, Trigger, and Filter, and to set Spaces' TriggerFilterID.

## The loop

1. **Preview** (dry-run — the default): `cub-scheduling guardrails install -o table` — prints the Trigger, which Spaces it would wire, which are already wired, and which it skips (with the reason).
2. **Install** with `--commit`: `cub-scheduling guardrails install --commit`. The Trigger is `Warn=true` (advisory). Promote it to blocking with `cub trigger update workload-toleration-needs-placement --space scheduling-policy --unwarn`.
3. **Check** what's flagged: `cub-scheduling guardrails status -o table`.
4. **Fix** flagged workloads via **scheduling-place**, then re-check.

## Stop conditions

- A Space is skipped (custom WhereTrigger / different TriggerFilterID) — report it; wiring means adding the guardrail Filter to that Space's existing selector, not overwriting.
- The user wants a blocking gate now — install advisory first, verify, then `--unwarn`.

## Tool boundary

Allowed: `guardrails install` (dry-run unless `--commit`), `status`. Not allowed: bypassing gates, `kubectl` mutations, applying to clusters. Fixing flagged workloads is **scheduling-place**.

## References

- `cub-scheduling guardrails --help`, `… install --help`, `… status --help`.
- Companion skills: **scheduling-place**, **scheduling-findings**, `triggers-and-applygates`.
