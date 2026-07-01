---
name: scheduling-findings
description: 'Produce severity-ranked Kubernetes workload placement findings across a ConfigHub fleet with the cub-scheduling CLI. v1 flags controllers that tolerate a taint but do not constrain where they land (no nodeSelector and no required node affinity), so they may schedule onto general nodes. Use for "what placement problems do we have?", "which workloads tolerate a taint but do not pin a node pool?", "placement findings for the ml component". Not for the raw placement report (use scheduling-audit), fixing placement (use scheduling-place), or availability spread (use workload-manager); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-scheduling --help) Bash(cub-scheduling * --help) Bash(cub auth status) Bash(cub-scheduling preflight) Bash(cub-scheduling findings) Bash(cub-scheduling findings *)
---

# scheduling-findings

Report placement anti-patterns across the fleet, most-severe first. Read-only.

## Why this matters

v1 flags the tractable, high-signal anti-pattern that needs no cluster facts: a controller that **tolerates a taint but doesn't constrain where it lands** (no nodeSelector and no required node affinity). Adding a toleration only *permits* scheduling onto tainted nodes; without a matching nodeSelector or node affinity the pod may still land on general nodes — usually not the intent. Analysis is over **ConfigHub-managed Units only**.

Checks that need cluster node-pool / taint facts (a nodeSelector for a pool no cluster advertises; a nodeSelector onto a tainted pool with no matching toleration) are deferred until those Target facts exist.

## When to use

- "What placement problems do we have?" → `findings`.
- "Which workloads tolerate a taint but don't pin a node pool?" → `findings` (the v1 analyzer).
- "Placement findings for the ml component / in prod." → `findings --component ml` / `--environment prod`.

## Do not load for

- The raw per-workload placement report — use **scheduling-audit** (`placement`).
- Fixing placement — use **scheduling-place**.
- Availability (pod anti-affinity / topology spread) — use **workload-manager**.

## Preflight gates

1. `cub-scheduling preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

- `cub-scheduling findings -o table` — severity-ranked placement findings (`--severity`, `--cluster`, `--namespace`).

Server-side `--where` + label shorthands scope the fleet; `--cluster` / `--namespace` are client-side.

## Stop conditions

- Zero findings — report the fleet is clean for that scope.
- The user wants to fix something — hand off to **scheduling-place**.

## Tool boundary

Read-only. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-scheduling findings --help`.
- Companion skills: **scheduling-audit**, **scheduling-place**.
