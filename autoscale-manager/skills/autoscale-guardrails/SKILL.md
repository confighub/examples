---
name: autoscale-guardrails
description: 'Install and inspect the autoscaling enforcement pack with the cub-autoscale CLI — an autoscale-policy Space with two Warn=true Triggers: a vet-cel rule that flags any HPA/ScaledObject that is pinned (min == max), and a vet-schemas rule that schema-validates every mutation (the post-convert check for convert-keda output, since keda.sh is in the schema catalog) — wired to in-scope Spaces via a shared Trigger Filter. guardrails install (dry-run by default) and status (Units with ApplyWarnings/Gates). Use for "enforce that no HPA is pinned", "make sure converted ScaledObjects pass schema validation", "which autoscalers are flagged?". Not for read-only findings (use autoscale-findings) or fixing autoscaling (use autoscale-edit).'
phase: act
allowed-tools: Bash(cub-autoscale --help) Bash(cub-autoscale * --help) Bash(cub auth status) Bash(cub-autoscale preflight) Bash(cub-autoscale guardrails) Bash(cub-autoscale guardrails *)
---

# autoscale-guardrails

Turn the autoscaling anti-patterns into **enforcement** — advisory ApplyWarnings (promotable to blocking ApplyGates) that fire in the normal apply pipeline.

- **`guardrails install`** — creates the `autoscale-policy` Space, two `Warn=true` Triggers, and a shared Trigger Filter, then wires in-scope Spaces to it. **Dry-run by default**; re-run with `--commit`.
  - `autoscaler-not-pinned` (`vet-cel`) — an HPA/ScaledObject must not have `min == max`.
  - `schema-valid` (`vet-schemas`) — a resource must pass Kubernetes/CRD schema validation. This is the **post-convert check** for `convert-keda`: a committed ScaledObject fires the Mutation Trigger and is validated against `keda.sh`'s schema.
- **`guardrails status`** — lists Units carrying autoscaling ApplyWarnings or ApplyGates.

## Why this matters

Both rules are plain **per-resource** checks — a single Unit answers each, so no annotate-then-validate is needed. The manager can't set warnings directly; only a failed Trigger can. `install` is conservative: it skips Spaces that already select Triggers their own way (a custom WhereTrigger or a different TriggerFilterID) rather than clobbering them — those are reported with the reason.

## When to use

- "Enforce that no HPA is ever pinned." → `guardrails install` (preview), then `--commit`.
- "Make sure converted ScaledObjects pass schema validation." → the `schema-valid` Trigger (installed by the same pack).
- "Which autoscalers are flagged?" → `guardrails status`.

## Do not load for

- Read-only findings — use **autoscale-findings**.
- Fixing autoscaling — use **autoscale-edit**.
- General Trigger/ApplyGate mechanics beyond this pack — use `triggers-and-applygates`.

## Preflight gates

1. `cub-autoscale preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. The user has permission to create the policy Space, Triggers, and Filter, and to set Spaces' TriggerFilterID.

## The loop

1. **Preview** (dry-run — the default): `cub-autoscale guardrails install -o table` — prints the Triggers, which Spaces it would wire, which are already wired, and which it skips (with the reason).
2. **Install** with `--commit`: `cub-autoscale guardrails install --commit`. The Triggers are `Warn=true` (advisory). Promote one to blocking with `cub trigger update <slug> --space autoscale-policy --unwarn`.
3. **Check** what's flagged: `cub-autoscale guardrails status -o table`.
4. **Fix** flagged autoscalers via **autoscale-edit**, then re-check.

## Stop conditions

- A Space is skipped (custom WhereTrigger / different TriggerFilterID) — report it; wiring means adding the guardrail Filter to that Space's existing selector, not overwriting.
- The user wants a blocking gate now — install advisory first, verify, then `--unwarn`.

## Tool boundary

Allowed: `guardrails install` (dry-run unless `--commit`), `status`. Not allowed: bypassing gates, `kubectl` mutations, applying to clusters. Fixing flagged autoscalers is **autoscale-edit**.

## References

- `cub-autoscale guardrails --help`, `… install --help`, `… status --help`.
- Companion skills: **autoscale-edit**, **autoscale-findings**, `triggers-and-applygates`.
