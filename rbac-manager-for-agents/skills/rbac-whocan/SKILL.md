---
name: rbac-whocan
description: 'Answer Kubernetes effective-access questions across a ConfigHub fleet with the cub-rbac CLI: "who can VERB RESOURCE?" and the inverse "what can SUBJECT do?". Use for "who can delete secrets in prod?", "who can exec into pods?", "what can the ci-deployer service account do?", "which subjects have access to configmaps in payments?", "can anyone create clusterrolebindings?". Resolves RoleBindings/ClusterRoleBindings through their roles (including ClusterRole aggregation) and honors namespace scope and resourceNames. Not for inventory/listing (use rbac-audit) or risk/hygiene findings (use rbac-findings); not for live cluster authorization (use kubectl auth can-i).'
phase: verify
allowed-tools: Bash(cub-rbac --help) Bash(cub-rbac * --help) Bash(cub auth status) Bash(cub-rbac preflight) Bash(cub-rbac who-can *) Bash(cub-rbac access *)
---

# rbac-whocan

Effective-access queries over the fleet's Kubernetes RBAC, computed from the config ConfigHub holds — no cluster access required. Two directions:

- **who-can** — "who can VERB RESOURCE?" → every subject, on which cluster, granted by which binding/role, in what namespace scope.
- **access** — "what can SUBJECT do?" → every role a subject holds, fleet-wide.

This is analysis-only; it never mutates.

## Why this matters

`cub-rbac` mirrors the Kubernetes API server's authorization rules over stored config: verb/resource/apiGroup wildcards, resource vs. subresource (`pods/exec`), `resourceNames`, `nonResourceURLs`, RoleBinding→ClusterRole resolution, namespace scoping, and ClusterRole aggregation to a fixed point. So "who can delete secrets in prod" is answered the way the cluster would, but across every cluster at once.

Only `cluster-admin` is credited among Kubernetes built-in ClusterRoles (its rules are known without a manifest). Bindings to `admin`/`edit`/`view`/`system:*` are surfaced by **rbac-findings**, not invented as matches here.

## When to use

- "Who can {get,list,create,update,delete,deletecollection} RESOURCE [in NAMESPACE]?"
- "Who can exec into pods?" → `who-can create pods/exec`.
- "Who can bind/escalate/impersonate?" / "who can create clusterrolebindings?"
- "What can SUBJECT do?" / "what roles does the ci-deployer service account hold?"
- "Which subjects can reach secrets in payments?"

## Do not load for

- Inventory / "list every binding" / "how many roles per cluster" (use **rbac-audit**).
- "Is this risky / any wildcards / orphans / cluster-admin?" (use **rbac-findings**).
- Live-cluster authorization checks — `kubectl auth can-i`.

## Preflight gates

1. `cub-rbac preflight` succeeds (cub installed, ConfigHub session valid). If not, ask the user to run `cub auth login` and retry.

## who-can — "who can VERB RESOURCE?"

```bash
cub-rbac who-can <verb> <resource> [flags]

cub-rbac who-can get secrets                         # core group, anywhere
cub-rbac who-can create pods/exec --namespace payments
cub-rbac who-can update deployments --api-group apps
cub-rbac who-can create clusterrolebindings --api-group rbac.authorization.k8s.io
```

- `<resource>` is the plural name, optionally with a subresource (`pods/log`, `*/scale`).
- `--api-group` — omit for the core group; set for named groups (`apps`, `rbac.authorization.k8s.io`, …). A core-group rule will NOT match a named-group query and vice versa, so get the group right.
- `--namespace` — restrict to grants effective in that namespace (ClusterRoleBindings still apply; RoleBindings only in their own namespace).
- `--name` — honor `resourceNames` restrictions for a specific object.

Each result row: cluster, subject, scope (namespace or `cluster-wide`), the role and binding, and the Unit. `cluster-admin` matches show `(builtin)`.

## access — "what can SUBJECT do?"

```bash
cub-rbac access <subject>

cub-rbac access User:alice@example.com
cub-rbac access Group:oidc:developers
cub-rbac access ServiceAccount:apps/ci-deployer    # ServiceAccount:namespace/name
```

Subject format is `Kind:Name`, or `ServiceAccount:namespace/name` for a ServiceAccount. ServiceAccounts are matched by namespace + name. To discover subject names first, use **rbac-audit** (`cub-rbac list`).

## Scoping & output

- Narrow the fleet with `--target-where` / `--space-where` (ConfigHub filter expressions), e.g. `--target-where "Slug LIKE 'prod-%'"`.
- JSON by default; `-o table` for humans. Pipe JSON into `jq` for grouping/counting.

## Interpreting results

- Empty result = no stored grant matches — but note this reflects ConfigHub config, not necessarily the live cluster, and excludes uncredited builtins (`admin`/`edit`/`view`). Say so when it matters.
- A `(builtin)` row means access via `cluster-admin`.
- An `access` row marked `(unresolved)` means the bound role isn't in the snapshot (possibly a builtin or an orphan — see **rbac-findings**).

## Stop conditions

- The user wants inventory or risk analysis — hand off to **rbac-audit** / **rbac-findings**.
- The user needs live cluster truth — point to `kubectl auth can-i`.

## Tool boundary

Read-only analysis. No mutations, no `kubectl`.

## References

- `cub-rbac who-can --help`, `cub-rbac access --help`.
- Companion skills: **rbac-audit** (inventory), **rbac-findings** (hygiene).
