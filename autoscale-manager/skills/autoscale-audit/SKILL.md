---
name: autoscale-audit
description: 'Inventory and report Kubernetes autoscaling stored in ConfigHub across a fleet with the cub-autoscale CLI — HorizontalPodAutoscalers and KEDA ScaledObjects: each one''s scale target, min/max replicas, and whether it is pinned (min == max). Use for "what autoscalers do we have?", "list our HPAs / ScaledObjects", "which workloads are autoscaled?", "per-cluster autoscaler counts", "is autoscaler X pinned?". Not for autoscaling findings/anti-patterns (use autoscale-findings), edits or HPA→KEDA conversion (use autoscale-edit), or live cluster state (use kubectl); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-autoscale --help) Bash(cub-autoscale * --help) Bash(cub auth status) Bash(cub-autoscale preflight) Bash(cub-autoscale snapshot) Bash(cub-autoscale snapshot *) Bash(cub-autoscale list) Bash(cub-autoscale list *)
---

# autoscale-audit

Report the autoscaling ConfigHub holds — HorizontalPodAutoscalers and KEDA ScaledObjects — across the fleet. Read-only.

## Why this matters

`cub-autoscale` loads a fleet snapshot of autoscalers and the workloads they target, so you can see at a glance which workloads scale, what their replica bounds are, and which autoscalers are **pinned** (`min == max`, so they can't actually scale). Clusters are ConfigHub Targets (the Space slug stands in for unbound Units). Analysis is over **ConfigHub-managed Units only**. Output is JSON by default; add `-o table` for humans.

## When to use

- "What autoscalers do we have?" / "audit autoscaling." → `list`.
- "List our HPAs / ScaledObjects." → `list` (KIND column).
- "Which workloads are autoscaled / not?" → `snapshot` (AUTOSCALED vs WORKLOADS).
- "Per-cluster autoscaler counts." → `snapshot`.
- "Is autoscaler X pinned?" → `list` (PINNED column).

## Do not load for

- Autoscaling anti-patterns, ranked (pinned, no-autoscaler, PDB-vs-minReplicas) — use **autoscale-findings**.
- Editing an HPA, converting HPA→KEDA, applying a profile — use **autoscale-edit**.
- Live cluster state (current replica count, HPA status) — `kubectl`.

## Preflight gates

1. `cub-autoscale preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

- `cub-autoscale snapshot -o table` — per-cluster HPA / ScaledObject / workload / autoscaled / PDB counts.
- `cub-autoscale list -o table` — per-autoscaler kind, scale target, min/max, and whether it's pinned.

### Scoping

Server-side `--where` plus label shorthands (`--component`, `--environment`, `--region`, `--owner`, `--layer`, `--variant`), AND-only.

## Stop conditions

- Snapshot empty — report it; suggest widening scope or checking `cub auth status`.
- The question is about anti-patterns or fixing autoscaling — hand off to **autoscale-findings** / **autoscale-edit**.

## Tool boundary

Read-only. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-autoscale snapshot --help`, `cub-autoscale list --help`.
- Companion skills: **autoscale-findings**, **autoscale-edit**.
