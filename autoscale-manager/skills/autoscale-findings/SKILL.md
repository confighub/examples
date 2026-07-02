---
name: autoscale-findings
description: 'Produce severity-ranked Kubernetes autoscaling findings across a ConfigHub fleet with the cub-autoscale CLI. Flags autoscalers that are pinned (min == max, can''t scale), workloads with no HPA/ScaledObject, and the cross-resource case where a PodDisruptionBudget''s minAvailable blocks all voluntary eviction at the autoscaler''s minReplicas. Use for "what autoscaling problems do we have?", "which HPAs can''t scale?", "which workloads lack an autoscaler?", "is any PDB blocking scale-down?". Not for the raw autoscaler inventory (use autoscale-audit), fixing autoscaling (use autoscale-edit), or live cluster state (use kubectl); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-autoscale --help) Bash(cub-autoscale * --help) Bash(cub auth status) Bash(cub-autoscale preflight) Bash(cub-autoscale findings) Bash(cub-autoscale findings *)
---

# autoscale-findings

Report autoscaling anti-patterns across the fleet, most-severe first. Read-only.

## Why this matters

Three high-signal checks that need no live cluster facts, over **ConfigHub-managed Units only**:

- **autoscaler-pinned** (medium) — an HPA/ScaledObject with `min == max`: it's an autoscaler that can't scale.
- **pdb-blocks-min-scale** (medium) — the cross-resource case: a PodDisruptionBudget whose `minAvailable` is `>=` the autoscaler's `minReplicas` (or `100%`), so at minimum scale no pod may be voluntarily evicted (node drains stall). This join of HPA `minReplicas` against PDB `minAvailable` is the check no single-resource validator makes.
- **no-autoscaler** (low) — a Deployment/StatefulSet no HPA or ScaledObject targets.

## When to use

- "What autoscaling problems do we have?" → `findings`.
- "Which HPAs can't actually scale?" → `findings` (autoscaler-pinned).
- "Is a PDB blocking scale-down anywhere?" → `findings` (pdb-blocks-min-scale).
- "Which workloads lack an autoscaler?" → `findings` (no-autoscaler).
- "Autoscaling findings for prod / the ml component." → `findings --environment prod` / `--component ml`.

## Do not load for

- The raw autoscaler inventory — use **autoscale-audit** (`list`, `snapshot`).
- Fixing autoscaling — use **autoscale-edit**.
- Live cluster state — `kubectl`.

## Preflight gates

1. `cub-autoscale preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

- `cub-autoscale findings -o table` — severity-ranked findings (`--min-severity low|medium|high`).

Server-side `--where` + label shorthands scope the fleet.

## Stop conditions

- Zero findings — report the fleet is clean for that scope.
- The user wants to fix something — hand off to **autoscale-edit**.

## Tool boundary

Read-only. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-autoscale findings --help`.
- Companion skills: **autoscale-audit**, **autoscale-edit**, **autoscale-guardrails**.
