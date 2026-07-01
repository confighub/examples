---
name: scheduling-audit
description: 'Inventory and report the placement of Kubernetes workloads stored in ConfigHub across a fleet with the cub-scheduling CLI — where each workload is allowed to land: nodeSelector, tolerations, and node affinity. Use for "where do our workloads schedule?", "which workloads pin to a node pool?", "what does workload X tolerate?", "which workloads are unconstrained?", "per-cluster placement counts". Not for placement anti-pattern findings (use scheduling-findings), fixes (use scheduling-place), pod anti-affinity / topology spread (that is availability, owned by workload-manager), or live cluster state (use kubectl); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-scheduling --help) Bash(cub-scheduling * --help) Bash(cub auth status) Bash(cub-scheduling preflight) Bash(cub-scheduling snapshot) Bash(cub-scheduling snapshot *) Bash(cub-scheduling list) Bash(cub-scheduling list *) Bash(cub-scheduling placement) Bash(cub-scheduling placement *)
---

# scheduling-audit

Report where the workloads ConfigHub holds are allowed to land — their nodeSelector, tolerations, and node affinity — across the fleet. Read-only.

## Why this matters

Placement is "which node a pod lands on." `cub-scheduling` loads a fleet snapshot of pod-bearing workloads and reports each one's placement, so you can see at a glance which workloads pin to a node pool, which tolerate taints, and which are unconstrained (land anywhere). Clusters are ConfigHub Targets (the Space slug stands in for unbound Units). Analysis is over **ConfigHub-managed Units only**. Output is JSON by default; add `-o table` for humans.

Note the distinction the report makes: **tolerations alone do not constrain placement** — they only permit scheduling onto tainted nodes. A workload is "constrained" only if it has a nodeSelector or a required node affinity.

## When to use

- "Where do our workloads schedule?" / "audit placement." → `placement`.
- "Which workloads pin to a node pool / set a nodeSelector?" → `placement` (NODESELECTOR column) or `snapshot`.
- "What does workload X tolerate?" → `placement --namespace X` / `--cluster X`.
- "Which workloads are unconstrained?" → `placement --unconstrained-only`.
- "Per-cluster placement counts." → `snapshot`.
- "List the workloads in cluster/namespace X." → `list`.

## Do not load for

- Placement anti-patterns, ranked — use **scheduling-findings**.
- Changing placement — use **scheduling-place**.
- Pod anti-affinity / topology spread (spreading a workload's own replicas) — that is *availability*, owned by **workload-manager**, not placement.
- Live cluster state — `kubectl`.

## Preflight gates

1. `cub-scheduling preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

- `cub-scheduling snapshot -o table` — per-cluster workload counts + how many set nodeSelector / tolerations / node affinity.
- `cub-scheduling placement -o table` — per-workload nodeSelector, tolerations, node affinity, and whether it's constrained (`--unconstrained-only`, `--cluster`, `--namespace`).
- `cub-scheduling list -o table` — the workload explorer (`--kind`, `--cluster`, `--namespace`).

### Scoping

Server-side `--where` plus label shorthands (`--component`, `--environment`, `--region`, `--owner`, `--layer`, `--variant`), AND-only. `--cluster` / `--namespace` are client-side display filters.

## Stop conditions

- Snapshot empty — report it; suggest widening scope or checking `cub auth status`.
- The question is about anti-patterns or fixing placement — hand off to **scheduling-findings** / **scheduling-place**.

## Tool boundary

Read-only. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-scheduling snapshot --help`, `cub-scheduling placement --help`, `cub-scheduling list --help`.
- Companion skills: **scheduling-findings**, **scheduling-place**.
