---
name: netpol-connectivity
description: 'Answer effective NetworkPolicy connectivity questions across a ConfigHub fleet with the cub-netpol CLI: who is allowed to reach a workload, and what a workload is allowed to reach, under the combined policy set. Use for "who can reach the payments pods?", "what can the frontend talk to?", "is checkoutservice allowed to reach the database?", "which workloads can reach redis?". Computes reachability from NetworkPolicy isolation + ingress/egress rules (additive OR semantics). Not for inventory/coverage (use netpol-audit) or anti-pattern findings (use netpol-findings); not for live traffic (use a CNI flow tool).'
phase: verify
allowed-tools: Bash(cub-netpol --help) Bash(cub-netpol * --help) Bash(cub auth status) Bash(cub-netpol preflight) Bash(cub-netpol who-can-reach *) Bash(cub-netpol reachable-from *) Bash(cub-netpol list *)
---

# netpol-connectivity

Answer "who can reach X" and "what can X reach" from the NetworkPolicy set, the way the cluster's additive policy model actually resolves it. Read-only.

## Why this matters

NetworkPolicy is additive and bidirectional: traffic A→B is allowed only if A's egress permits B **and** B's ingress permits A, and a pod is isolated the moment any policy selects it. Reading that off raw YAML by hand is error-prone. `cub-netpol` builds a connectivity model over the fleet snapshot — selector matching, namespace scoping (including the implicit `kubernetes.io/metadata.name` label), and isolation inference — and answers reachability directly.

## When to use

- "Who can reach the payments pods / cartservice / redis?" → `who-can-reach <workload>`.
- "What can the frontend talk to?" / "what can checkoutservice reach?" → `reachable-from <workload>`.
- "Is A allowed to reach B?" — run `who-can-reach B` (or `reachable-from A`) and check for the other.
- Verifying that a default-deny + allow set produced the intended least-privilege graph.

## Do not load for

- Inventory / coverage gaps — use **netpol-audit**.
- Anti-patterns (allow-all, metadata egress, asymmetry) — use **netpol-findings**.
- Live observed traffic — that's a CNI flow/observability tool, not config analysis.

## Preflight gates

1. `cub-netpol preflight` succeeds. If not, ask the user to run `cub auth login` and retry.

## The toolkit

```bash
# Sources allowed to reach a destination workload (its effective ingress, ∩ each source's egress).
cub-netpol who-can-reach cartservice --cluster prod-cluster -o table

# Destinations a source workload is allowed to reach (its effective egress, ∩ each dest's ingress).
cub-netpol reachable-from frontend --cluster prod-cluster -o table
```

- Resolve the workload by name; disambiguate with `--cluster`, `--namespace`, `--kind` when a name is not unique (the CLI errors and lists matches if it is ambiguous).
- An **open** cluster (no policies) returns a full mesh — every workload reaches every other. That is the correct "no isolation" answer, and a signal that a default-deny is missing (see **netpol-audit** / **netpol-fix**).

### Limitations (state them when relevant)

- ipBlock peers do not match pod-to-pod traffic (they describe external CIDRs).
- Port constraints are not considered in the reachability boolean — a flow allowed on any port counts as reachable.
- Reachability is over ConfigHub-managed Units only.

## Stop conditions

- The named workload isn't found / is ambiguous — report the CLI's message and ask for `--cluster`/`--namespace`.
- The user wants to *change* connectivity (add an allow, lock down) — hand off to **netpol-fix**.

## Tool boundary

Read-only. Never `kubectl` to answer a ConfigHub-config question.

## References

- `cub-netpol who-can-reach --help`, `cub-netpol reachable-from --help`.
- Companion skills: **netpol-audit**, **netpol-findings**, **netpol-fix**.
