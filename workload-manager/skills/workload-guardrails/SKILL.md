---
name: workload-guardrails
description: 'Install and operate the workload-readiness enforcement pack with the cub-workload CLI — a workload-policy Space of Warn=true vet-cel Triggers (containers set a memory limit, run as non-root, set terminationMessagePolicy) plus an annotate-then-validate Trigger for cross-Unit PodDisruptionBudget coverage, wired to in-scope Spaces via a shared Trigger Filter. guardrails install (dry-run by default), status (Units with ApplyWarnings/Gates), annotate (write the PDB-coverage finding onto uncovered workloads). Use for "enforce workload readiness", "block workloads with no memory limit", "warn on workloads with no PDB", "which workloads are flagged?". Not for read-only scoring (use workload-audit / workload-findings) or fixing workloads (use workload-harden / workload-fleet).'
phase: act
allowed-tools: Bash(cub-workload --help) Bash(cub-workload * --help) Bash(cub auth status) Bash(cub-workload preflight) Bash(cub-workload guardrails) Bash(cub-workload guardrails *)
---

# workload-guardrails

Turn workload-readiness policy into **enforcement** — advisory ApplyWarnings (promotable to blocking ApplyGates) that fire in the normal apply pipeline. Three subcommands:

- **`guardrails install`** — creates the `workload-policy` Space, four `Warn=true` `vet-cel` Triggers, and a shared Trigger Filter, then wires in-scope Spaces to it. **Dry-run by default**; re-run with `--commit`.
- **`guardrails status`** — lists Units carrying workload-readiness ApplyWarnings or ApplyGates.
- **`guardrails annotate`** — writes the `workload.confighub.com/pdb-coverage` finding onto each uncovered multi-replica workload Unit (the producing half of annotate-then-validate). Dry-run unless `--commit --change-desc`.

## Why this matters

The manager can't set ApplyWarnings directly — only a failed Trigger can. Three of the rules are plain **per-resource `vet-cel`** (a single Unit answers them): `workload-has-limits`, `workload-runs-nonroot`, `workload-termination-message-policy`. The fourth, **`workload-pdb-coverage`**, is the one property `vet-cel` can't see under one-resource-per-Unit — whether a *matching* PDB exists in another Unit — so it uses **annotate-then-validate**: `annotate` computes the cross-Unit coverage finding and stamps it on the workload Unit, and the Trigger turns that annotation into a warning. `install` is conservative: it skips Spaces that already select Triggers their own way (a custom WhereTrigger or a different TriggerFilterID) rather than clobbering them.

## When to use

- "Enforce workload readiness across the fleet." → `guardrails install` (preview), then `--commit`.
- "Block / warn on workloads with no memory limit or running as root." → the `install`ed vet-cel Triggers.
- "Warn on workloads with no PDB." → `guardrails install` + `guardrails annotate --commit`.
- "Which workloads are flagged?" → `guardrails status`.

## Do not load for

- Read-only scoring — use **workload-audit** / **workload-findings** / **workload-availability**.
- Fixing a workload — use **workload-harden** (single) / **workload-fleet** (bulk).
- General Trigger/ApplyGate mechanics beyond this pack — use `triggers-and-applygates`.

## Preflight gates

1. `cub-workload preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. The user has permission to create the policy Space, Triggers, and Filter, and to set Spaces' TriggerFilterID.

## The loop

1. **Preview** the pack and wiring (dry-run — the default):
   ```bash
   cub-workload guardrails install -o table
   ```
   It prints the Triggers, which Spaces it would wire, which are already wired, and which it skips (with the reason).
2. **Install** with `--commit`:
   ```bash
   cub-workload guardrails install --commit
   ```
   Triggers are `Warn=true` (advisory). Promote one to blocking later with `cub trigger update <slug> --space workload-policy --unwarn`.
3. **Feed the annotate-then-validate rule** (for PDB coverage):
   ```bash
   cub-workload guardrails annotate                       # dry-run: which workloads would be annotated
   cub-workload guardrails annotate --commit --change-desc "Annotate uncovered workloads. User prompt: ..."
   ```
   Re-run after PDBs are added so the annotation (a cached finding) stays fresh.
4. **Check** what's flagged: `cub-workload guardrails status -o table`.
5. **Fix** flagged workloads via **workload-harden** / **workload-fleet**, then re-check.

## Stop conditions

- A Space is skipped (custom WhereTrigger / different TriggerFilterID) — report it; wiring it means adding the guardrail Filter to that Space's existing selector (a `triggers-and-applygates` task), not overwriting.
- The user wants a blocking gate now — install advisory first, verify, then `--unwarn` the specific Trigger.

## Tool boundary

Allowed: `guardrails install` (dry-run unless `--commit`), `status`, `annotate` (dry-run unless `--commit --change-desc`). Not allowed: bypassing gates, `kubectl` mutations, applying to clusters. Fixing flagged workloads is **workload-harden** / **workload-fleet**.

## References

- `cub-workload guardrails --help`, `… install --help`, `… status --help`, `… annotate --help`.
- Companion skills: **workload-harden**, **workload-fleet**, **workload-availability**, `triggers-and-applygates`.
