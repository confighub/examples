---
name: workload-availability
description: 'Report the disruption-survival posture of multi-replica Kubernetes workloads across a ConfigHub fleet with the cub-workload CLI — PodDisruptionBudget coverage (does a matching PDB exist in some other Unit?), the minAvailable>=replicas / maxUnavailable:0 eviction-lock footgun, and pod anti-affinity / topology spread presence. Use for "which multi-replica workloads have no PDB?", "is workload X covered by a PodDisruptionBudget?", "which workloads could lose every replica to one node/zone failure?", "does this workload have anti-affinity?". Not for security/resources/probes scoring (use workload-audit), ranked findings (use workload-findings), or fixes (use workload-harden); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-workload --help) Bash(cub-workload * --help) Bash(cub auth status) Bash(cub-workload preflight) Bash(cub-workload availability) Bash(cub-workload availability *) Bash(cub-workload snapshot) Bash(cub-workload snapshot *)
---

# workload-availability

Report which multi-replica workloads can survive a voluntary disruption (a node drain, a zone loss): whether a **PodDisruptionBudget** matches them, whether that PDB is so tight it blocks *all* evictions, and whether they declare pod anti-affinity or topology spread. Read-only.

## Why this matters

**PDB coverage is a cross-Unit property.** Under one-resource-per-Unit, a workload and its PodDisruptionBudget live in *separate* Units, so a per-Unit validating Trigger sees the workload alone and cannot tell whether a matching PDB exists — it structurally false-positives. `cub-workload` does the selector→pod join over the whole fleet snapshot (matching a PDB's `selector` against a workload's pod-template labels within its namespace + cluster) and reports true coverage. It also flags the highest-signal PDB footgun — `minAvailable >= replicas` or `maxUnavailable: 0`, which makes node drains hang forever — which needs the PDB read *together with* the workload's replica count. Analysis is over **ConfigHub-managed Units only**.

## When to use

- "Which multi-replica workloads have no matching PDB?" → `availability` (or `--issues-only`).
- "Is workload X covered by a PodDisruptionBudget?" → `availability --namespace X`.
- "Which workloads could lose every replica to one node/zone failure?" → `availability` (SPREAD = no).
- "Any PDBs that block all evictions?" → `availability` (BLOCKS-EVICT = yes).

## Do not load for

- Security context / resources / probes scoring — use **workload-audit** (`readiness`).
- The ranked list of everything wrong — use **workload-findings**.
- Fixing gaps (ensure-pdb / ensure-spread) — use **workload-harden**.
- Live cluster state — `kubectl`.

## Preflight gates

1. `cub-workload preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

### Availability report — `cub-workload availability`

Per multi-replica workload: replicas, matching PDB (or MISSING), whether the PDB blocks all evictions, and whether spread is present. Single-replica workloads and DaemonSet / Job / CronJob / Pod are out of scope (a PDB gains a single instance nothing).

```bash
cub-workload availability -o table
cub-workload availability --issues-only -o table         # just the gaps
cub-workload availability --cluster prod-cluster --namespace checkout
```

### Scoping

Same server-side `--where` + label shorthands (`--component`, `--environment`, …) as the other read commands; `--cluster` / `--namespace` are client-side display filters.

## Stop conditions

- No multi-replica workloads in scope — report it (nothing to cover).
- The question is about non-availability readiness or a ranked finding list — hand off to **workload-audit** / **workload-findings**.

## Tool boundary

Read-only. Fixing coverage (`ensure-pdb`) or spread (`ensure-spread`) lives in **workload-harden**. Never use `kubectl`.

## References

- `cub-workload availability --help`.
- Companion skills: **workload-audit**, **workload-findings**, **workload-harden**.
