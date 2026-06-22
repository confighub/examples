---
name: rbac-audit
description: 'Inventory and explore Kubernetes RBAC config stored in ConfigHub across the fleet, using the cub-rbac CLI. Use for "what RBAC do we have?", "list every ClusterRoleBinding", "which clusters have RBAC and how much?", "show all Roles in the payments namespace", "audit our RBAC footprint", "how many service accounts per cluster?". Not for "who can do X" effective-access queries (use rbac-whocan) or hygiene/risk analysis (use rbac-findings); not for live cluster RBAC (use kubectl); not for ConfigHub-entity permissions (out of scope).'
phase: verify
allowed-tools: Bash(cub-rbac --help) Bash(cub-rbac * --help) Bash(cub auth status) Bash(cub-rbac preflight) Bash(cub-rbac snapshot) Bash(cub-rbac snapshot *) Bash(cub-rbac list) Bash(cub-rbac list *)
---

# rbac-audit

Inventory and browse the Kubernetes RBAC config (Role, ClusterRole, RoleBinding, ClusterRoleBinding, ServiceAccount) that ConfigHub holds across the fleet. This is the "what do we have / show me" surface; it never mutates.

## Why this matters

ConfigHub stores RBAC as data, so the whole fleet's RBAC is queryable like a database instead of `kubectl get` against each cluster. `cub-rbac` loads a fleet snapshot (every Kubernetes/YAML Unit's RBAC resources, joined with Space/Target metadata) and answers inventory questions over it. Clusters are ConfigHub Targets (the Space slug stands in for unbound "paper cluster" Units). Output is JSON by default for piping into `jq`; add `-o table` for humans.

## When to use

- "What RBAC do we have across the fleet?" / "audit our RBAC footprint."
- "List every ClusterRoleBinding" / "show all Roles in namespace X" / "all ServiceAccounts on cluster Y."
- "How many roles/bindings/service accounts per cluster?" / "which clusters carry RBAC?"
- "How many Units are gated or unapplied?" (per-cluster, from `snapshot`).

## Do not load for

- "Who can VERB RESOURCE?" or "what can SUBJECT do?" — effective-access (use **rbac-whocan**).
- "Is our RBAC risky / any wildcards / cluster-admin / orphans?" — hygiene (use **rbac-findings**).
- Live cluster RBAC not in ConfigHub — `kubectl get clusterrolebindings`.
- ConfigHub's own per-entity permissions — out of scope for cub-rbac.

## Preflight gates

1. `cub-rbac preflight` succeeds — it confirms the `cub` CLI is installed and the ConfigHub session is valid against the server. If it fails, ask the user to run `cub auth login` (an interactive browser sign-in an agent cannot complete), then retry.

## The toolkit

### Fleet inventory — `cub-rbac snapshot`

Per-cluster counts of Roles / ClusterRoles / RoleBindings / ClusterRoleBindings / ServiceAccounts, plus Units, gated Units, and unapplied Units. Canonical base/policy Spaces are excluded.

```bash
cub-rbac snapshot                 # JSON
cub-rbac snapshot -o table        # human table + totals line
```

### Explorer — `cub-rbac list`

Every RBAC resource across the fleet with its cluster, Space, and Unit. Canonical definitions are included and flagged.

```bash
cub-rbac list                                   # all resources, JSON
cub-rbac list --kind ClusterRoleBinding         # one kind
cub-rbac list --cluster prod-use2-oci -o table  # one cluster
cub-rbac list --kind Role --namespace payments  # kind + namespace
```

Filters: `--kind` (Role | ClusterRole | RoleBinding | ClusterRoleBinding | ServiceAccount), `--cluster` (Target or Space slug), `--namespace`.

### Scoping the fleet

Both commands accept ConfigHub filter expressions to narrow scope:

- `--target-where "Slug LIKE 'prod-%'"` — scope deployed Units by Target.
- `--space-where "Labels.Environment = 'prod'"` — scope untargeted base Units by Space.

Prefer scoping server-side with these over post-filtering large JSON.

## Working with output

JSON is the default — pipe into `jq` for counts, grouping, and extraction. Examples:

```bash
# Clusters sorted by binding count
cub-rbac snapshot | jq -r '.clusters | sort_by(-.roleBindings-.clusterRoleBindings)[] | "\(.cluster)\t\(.roleBindings+.clusterRoleBindings)"'

# Every ClusterRoleBinding name on one cluster
cub-rbac list --kind ClusterRoleBinding --cluster prod-use2-oci | jq -r '.[].name'
```

## Stop conditions

- Snapshot is empty (no Kubernetes/YAML Units in scope) — report that and suggest widening scope or checking `cub auth status` / org context.
- The question is about effective access or risk — hand off to **rbac-whocan** / **rbac-findings**.

## Tool boundary

Read-only. `cub-rbac` here is inventory/explorer only; it makes no changes. Mutations are a separate (future) write skill. Never reach for `kubectl` to answer a ConfigHub-state question.

## References

- `cub-rbac snapshot --help`, `cub-rbac list --help` — full flag reference.
- Companion skills: **rbac-whocan** (effective access), **rbac-findings** (hygiene).
