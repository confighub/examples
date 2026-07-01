---
name: rbac-findings
description: 'Surface Kubernetes RBAC hygiene and risk issues across a ConfigHub fleet with the cub-rbac CLI: wildcard permissions, privilege-escalation verbs, risky grants (secrets/exec/webhooks/CRDs), cluster-admin bindings, orphaned bindings, and unbound service accounts. Use for "audit our RBAC hygiene", "any wildcard roles?", "who has cluster-admin?", "find privilege escalation", "any orphaned role bindings?", "show RBAC risks in prod". Analysis-only — enforcement is server-side via Triggers/ApplyGates. Not for inventory/listing (use rbac-audit) or effective-access who-can queries (use rbac-whocan); not for live cluster scanning (use a cluster scanner).'
phase: verify
allowed-tools: Bash(cub-rbac --help) Bash(cub-rbac * --help) Bash(cub auth status) Bash(cub-rbac preflight) Bash(cub-rbac findings) Bash(cub-rbac findings *)
---

# rbac-findings

Run RBAC hygiene analyzers over the fleet's stored Kubernetes config and report issues, severity-ranked. Analysis-only: it never mutates and never enforces.

## Why this matters

ConfigHub stores RBAC as data, so hygiene checks run across every cluster at once from one snapshot — no per-cluster scanning. These findings are the *advisory* complement to enforcement: blocking bad RBAC at write time is done server-side with Triggers + ApplyGates (a separate concern). Canonical base/policy Spaces are excluded so definitions don't produce phantom findings.

## The analyzers

| analyzer | severity | flags |
|---|---|---|
| `wildcard-rules` | high | `*` in a role's verbs, resources, or apiGroups |
| `privilege-escalation-verbs` | high | `escalate` / `bind` / `impersonate` |
| `risky-grants` | medium | secrets, `pods/exec`+`pods/attach`, webhook/CRD writes |
| `cluster-admin-bindings` | high (CRB) / medium (RB) | bindings to cluster-admin or an equivalent custom superuser role |
| `orphaned-bindings` | medium | binding whose role does not exist on the cluster (builtins excluded) |
| `unbound-service-accounts` | low | ServiceAccount with no bindings in the snapshot |

## When to use

- "Audit our RBAC hygiene" / "show RBAC risks [in prod]."
- "Any wildcard roles?" / "find privilege escalation" / "who has cluster-admin?"
- "Any orphaned role bindings?" / "unused service accounts?"

## Do not load for

- Inventory / "list bindings" / "counts per cluster" (use **rbac-audit**).
- "Who can VERB RESOURCE?" / "what can SUBJECT do?" (use **rbac-whocan**).
- Actually blocking bad RBAC at write time — that's server-side Triggers/ApplyGates (a write-side concern), not this read-only analyzer.
- Live cluster scanning — use a dedicated cluster scanner.

## Preflight gates

1. `cub-rbac preflight` succeeds (cub installed, ConfigHub session valid). If not, ask the user to run `cub auth login` and retry.

## Usage

```bash
cub-rbac findings                          # all findings, JSON, severity-sorted
cub-rbac findings -o table                 # human table
cub-rbac findings --severity high          # high only
cub-rbac findings --analyzer wildcard-rules
cub-rbac findings --environment prod                    # scope to prod (label shorthand)
cub-rbac findings --where "Target.ProviderType = 'OCI'" # scope by any Unit/Space/Target attribute
```

Filters: `--severity` (high | medium | low), `--analyzer` (e.g. `cluster-admin-bindings`). Scope the fleet with a single Unit `--where` filter, or the label shorthands `--component` / `--environment` / `--region` / `--owner` / `--layer` / `--variant` (each expands to `Space.Labels.<Key> = '<value>'`). ConfigHub `where` is flat AND-only — no parentheses, no OR; the shorthands AND onto any `--where`.

Each finding row: severity, analyzer, cluster, resource kind, resource (namespace/name), the Unit, and a message explaining the issue and remediation.

## Triage workflow

1. Start with `cub-rbac findings -o table` for the ranked overview, or JSON for counts:
   ```bash
   cub-rbac findings | jq -r 'group_by(.severity)[] | "\(.[0].severity): \(length)"'
   cub-rbac findings | jq -r '[.[].analyzer]|group_by(.)|.[]|"\(length)\t\(.[0])"'
   ```
2. Drill into the highest severity first: `cub-rbac findings --severity high`.
3. For a flagged binding/role, pivot to **rbac-whocan** to see who it actually grants, and to **rbac-audit** to inspect the resource.
4. Report findings with their Unit so the user knows where to remediate (the edit itself is a future write skill / `cub` mutation, not this skill).

## Interpreting results

- Findings reflect ConfigHub's stored config, not live cluster state.
- `cluster-admin-bindings` includes custom roles whose effective rules are `*/*/*` (true superusers), not just literal cluster-admin.
- `orphaned-bindings` excludes Kubernetes builtins (`admin`/`edit`/`view`/`system:*`) and `cluster-admin`, which exist without a stored manifest.

## Stop conditions

- Zero findings — report a clean bill for the scope analyzed (note the scope).
- The user wants to *fix* a finding — that's a mutation; hand off to the write path (`cub` / future rbac-edit skill), don't edit here.

## Tool boundary

Read-only analysis. No mutations, no enforcement, no `kubectl`.

## References

- `cub-rbac findings --help`.
- Companion skills: **rbac-audit** (inventory), **rbac-whocan** (effective access).
