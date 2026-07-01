---
name: workload-audit
description: 'Inventory Kubernetes workloads stored in ConfigHub across the fleet and score their production-readiness (security context, resources/limits, probes, operational hygiene) with the cub-workload CLI. Use for "what workloads do we have?", "which workloads run as root / have no limits / no probes?", "what is the readiness of workload X?", "per-cluster workload counts", "score the dev workloads". Not for PodDisruptionBudget coverage / anti-affinity (use workload-availability), ranked findings (use workload-findings), fixes (use workload-harden), or live cluster state (use kubectl); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-workload --help) Bash(cub-workload * --help) Bash(cub auth status) Bash(cub-workload preflight) Bash(cub-workload snapshot) Bash(cub-workload snapshot *) Bash(cub-workload list) Bash(cub-workload list *) Bash(cub-workload readiness) Bash(cub-workload readiness *)
---

# workload-audit

Inventory the workloads ConfigHub holds across the fleet and report a **per-workload production-readiness scorecard** — security context, resource requests/limits, probes, and operational hygiene, each scored pass / warn / fail. This is the "what do we have / how healthy is it" surface; it never mutates.

## Why this matters

Per-object validators (kube-score, kube-linter) score one object, and — wired as a per-Unit ConfigHub Trigger under one-resource-per-Unit — see only a lone workload Unit. `cub-workload` loads a fleet snapshot (workloads + PodDisruptionBudgets, joined with Space/Target metadata) and scores every workload, so you get a fleet-wide scorecard, not a one-object check. Clusters are ConfigHub Targets (the Space slug stands in for unbound Units). Analysis is over **ConfigHub-managed Units only**. Output is JSON by default; add `-o table` for humans.

## When to use

- "What workloads do we have?" / "audit our workload posture." → `snapshot`, `list`.
- "Which workloads run as root / have no memory limit / no readiness probe?" → `readiness` (optionally `--dimension security|resources|probes|hygiene`).
- "What's the readiness of workload X?" → `readiness --namespace X` or `--cluster X`.
- "Just the problem workloads" → `readiness --failing-only`.
- "Per-cluster workload counts" → `snapshot`.

## Do not load for

- "Which multi-replica workloads have no PDB / no anti-affinity?" — availability (use **workload-availability**).
- "Give me the ranked list of everything wrong" — governance findings (use **workload-findings**).
- Fixing gaps (harden / set-resources / …) — use **workload-harden**.
- Live cluster state not in ConfigHub — `kubectl get deploy`.

## Preflight gates

1. `cub-workload preflight` succeeds (the ConfigHub session is valid against the server). If it fails, ask the user to run `cub auth login` (an interactive browser sign-in an agent cannot complete), then retry.

## The toolkit

### Fleet inventory — `cub-workload snapshot`

Per-cluster counts of workloads and PodDisruptionBudgets, plus Units, gated, and unapplied. Canonical base/policy Spaces are excluded.

```bash
cub-workload snapshot -o table
```

### Readiness scorecard — `cub-workload readiness`

Per workload: security / resources / probes / hygiene → pass / warn / fail, with the worst as the overall. In table mode, warn+fail workloads list their specific issues.

```bash
cub-workload readiness -o table
cub-workload readiness --failing-only -o table          # just the problems
cub-workload readiness --dimension security --cluster prod-cluster
```

### Explorer — `cub-workload list`

Every workload and PDB with its cluster, Space, and Unit.

```bash
cub-workload list --kind Deployment -o table
cub-workload list --cluster dev-cluster --namespace apptique
```

### Scoping the fleet

Scope server-side with a single Unit `--where` predicate — one Unit-level filter can reference Unit, Space, and Target metadata. Use the shorthands `--component`, `--environment`, `--region`, `--owner`, `--layer`, `--variant` (each compiles to `Space.Labels.<Key> = '<value>'`), AND-joined with any raw `--where`. ConfigHub `where` is flat AND-only — no parentheses, no `OR`. `--cluster` / `--namespace` are client-side display filters over the fetched snapshot.

## Stop conditions

- Snapshot empty (no Kubernetes/YAML workload Units in scope) — report it; suggest widening scope or checking `cub auth status` / org context.
- The question is about PDB coverage or a ranked finding list — hand off to **workload-availability** / **workload-findings**.

## Tool boundary

Read-only inventory + readiness scorecard. Fixing gaps lives in the write skills. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-workload snapshot --help`, `cub-workload readiness --help`, `cub-workload list --help`.
- Companion skills: **workload-availability**, **workload-findings**.
