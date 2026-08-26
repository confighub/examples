---
name: namespace-audit
description: 'Inventory and audit Kubernetes namespaces stored in ConfigHub across the fleet, using the cub-namespace CLI — the Namespace object, its pod-security labels, and duplicate-name collisions. Use for "what namespaces do we have?", "which namespaces are missing a Namespace object or pod-security labels?", "is this namespace''s envelope complete?", "per-cluster namespace/workload counts", "list the namespaces in apptique". Not for NetworkPolicy coverage (use netpol-audit) or RBAC (use rbac-audit). Not for cross-variant consistency (use namespace-consistency), ranked governance findings (use namespace-findings), or live cluster state (use kubectl); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-namespace --help) Bash(cub-namespace * --help) Bash(cub auth status) Bash(cub-namespace preflight) Bash(cub-namespace snapshot) Bash(cub-namespace snapshot *) Bash(cub-namespace list) Bash(cub-namespace list *) Bash(cub-namespace envelope) Bash(cub-namespace envelope *)
---

# namespace-audit

Inventory the namespaces ConfigHub holds across the fleet and report **envelope completeness** — which namespaces have a Namespace object and pod-security labels, and which are missing members. This is the "what do we have / where are the gaps" surface; it never mutates.

NetworkPolicy coverage is **network-policy-manager**'s subject and RBAC is **rbac-manager-for-agents**'s.

## Why this matters

A runtime tenancy controller (Capsule, the retired HNC) governs one cluster; a per-resource validator sees one object. Neither can answer *does every namespace across the fleet carry its full policy envelope?* — that is a property of the whole set of resources in a namespace, joined across types, over the fleet's source of record. `cub-namespace` loads a fleet snapshot (Namespaces + the workloads occupying them, joined with Space/Target metadata) and reasons over it. Clusters are ConfigHub Targets; Units whose Space has no release Target group under a single `None` cluster. Analysis is over **ConfigHub-managed Units only**. Output is JSON by default; add `-o table` for humans.

## When to use

- "What namespaces do we have?" / "audit our namespace governance."
- "Which namespaces are missing a Namespace object or pod-security labels?" → `envelope`.
- "Is namespace X's envelope complete?" → `envelope --namespace X`.
- "Per-cluster counts of namespaces and workloads" → `snapshot`.
- "List the namespaces (or the workloads in one) in cluster X" → `list`.

## Do not load for

- "Is a component's namespace the same across dev/prod?" — cross-variant consistency (use **namespace-consistency**).
- "Give me the ranked list of everything wrong" — governance findings (use **namespace-findings**).
- Live cluster state not in ConfigHub — `kubectl get ns`.
- Fixing gaps (scaffold / backfill) — that is a later write skill.

## Preflight gates

1. `cub-namespace preflight` succeeds (the ConfigHub session is valid against the server). If it fails, ask the user to run `cub auth login` (an interactive browser sign-in an agent cannot complete), then retry.

## The toolkit

### Fleet inventory — `cub-namespace snapshot`

Per-cluster counts of Namespace and workload, plus Units, gated, and unapplied. Canonical base/policy Spaces are excluded.

```bash
cub-namespace snapshot -o table
```

### Envelope completeness — `cub-namespace envelope`

Per namespace: pod-security enforce level and the list of missing members. Also flags duplicate Namespace objects colliding on name + Target. This is the headline audit.

```bash
cub-namespace envelope -o table
cub-namespace envelope --incomplete-only -o table   # just the gaps
cub-namespace envelope --namespace payments
```

### Explorer — `cub-namespace list`

Every Namespace and the workloads occupying it, with its cluster, Space, and Unit.

```bash
cub-namespace list --kind Namespace -o table
cub-namespace list --cluster prod-cluster --namespace apptique
```

### Scoping the fleet

Scope server-side with a single Unit `--where` predicate — one Unit-level filter can reference Unit, Space, and Target metadata, so prefer it over post-filtering large JSON. Example: `--where "Target.ProviderType = 'OCI'"` (the ProviderType recommended for ArgoCD/Flux) or `--where "Space.Slug LIKE 'apptique-%'"`.

For the standard Space labels, use the shorthands `--component`, `--environment`, `--region`, `--owner`, `--layer`, `--variant` (each compiles to `Space.Labels.<Key> = '<value>'`), AND-joined with any raw `--where`. ConfigHub `where` is flat AND-only — no parentheses, no `OR` (a parenthesized clause fails with `invalid attribute name`); express alternatives as separate runs.

`--cluster` / `--namespace` are client-side display filters over the fetched snapshot, not server-side scope. Tip: bind base/template Units to a Noop-ProviderType dummy Target (a no-apply, server-hosted bridge) so every Unit is targeted and `Target.Slug` is a consistent grouping key.

## Stop conditions

- Snapshot empty (no Kubernetes/YAML Units in scope) — report it; suggest widening scope or checking `cub auth status` / org context.
- The question is about cross-variant consistency or a ranked finding list — hand off to **namespace-consistency** / **namespace-findings**.

## Tool boundary

Read-only inventory + envelope completeness. Fixing gaps (scaffold / backfill / enforce) lives in the write skills. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-namespace snapshot --help`, `cub-namespace envelope --help`, `cub-namespace list --help`.
- Companion skills: **namespace-consistency**, **namespace-findings**.
