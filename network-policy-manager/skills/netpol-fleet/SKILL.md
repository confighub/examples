---
name: netpol-fleet
description: 'Fleet-wide NetworkPolicy operations in ConfigHub with the cub-netpol CLI: create a default-deny for every uncovered namespace at once, and promote a NetworkPolicy baseline from a base Space to its downstream cluster Spaces. Use for "default-deny every namespace that lacks one", "remediate all our coverage gaps", "roll the network-policy baseline out to all clusters", "which policy Units are behind their upstream?". Dry-run by default; requires --commit with --change-desc; Units are created/upgraded but NOT applied. Not for a single namespace/policy (use netpol-fix), inventory/findings (use netpol-audit/netpol-findings), or installing enforcement (use netpol-guardrails).'
phase: act
allowed-tools: Bash(cub-netpol --help) Bash(cub-netpol * --help) Bash(cub auth status) Bash(cub-netpol preflight) Bash(cub-netpol coverage *) Bash(cub-netpol findings *) Bash(cub-netpol fleet default-deny *) Bash(cub-netpol promote *)
---

# netpol-fleet

Fleet-wide remediation and promotion. Bulk-close coverage gaps, and propagate a baseline across cluster Spaces. **Dry-run by default**; nothing is written until `--commit` with a `--change-desc`, and Units are created/upgraded but **not applied** to a cluster.

## Why this matters

`netpol-fix` closes one gap; this closes them all at once, driven directly by the coverage analysis, and propagates a shared baseline through ConfigHub's variant model — the fleet-scale version of the same config-as-data, no-drift fix.

## When to use

- "Default-deny every namespace that lacks one." / "remediate all our coverage gaps." → `fleet default-deny`.
- "Roll the NetworkPolicy baseline out to all clusters." / "upgrade the downstreams to the base." → `promote`.
- "Which policy Units are behind their upstream?" → `promote` (dry-run).

## Do not load for

- A single namespace or policy — use **netpol-fix**.
- Inventory / findings — **netpol-audit**, **netpol-findings**.
- Installing enforcement Triggers — **netpol-guardrails**.
- Applying to clusters — **cub-apply**.

## Preflight gates

1. `cub-netpol preflight` succeeds. If not, ask the user to run `cub auth login` and retry.
2. For `promote`: the downstream Spaces are variant-linked to an upstream (created via `cub variant create`). With no upstream links, `promote` correctly reports nothing to do.

## The toolkit

### Bulk remediation — `cub-netpol fleet default-deny`

Finds every namespace with workloads but no default-deny ingress (the coverage gap) and authors a default-deny Unit for each, in its workloads' Space. Idempotent — covered namespaces are skipped.

```bash
cub-netpol fleet default-deny -o table                       # dry-run: what would be created
cub-netpol fleet default-deny --egress                        # also deny egress, allowing DNS
cub-netpol fleet default-deny --cluster prod-cluster \
  --commit --change-desc "Bulk default-deny for uncovered namespaces. User prompt: ..."
```

### Promotion — `cub-netpol promote`

Override-preserving upgrade of Units behind their upstream (the variant-propagation path: a baseline in a base Space flows to the cluster Spaces cloned from it, keeping per-Space customizations).

```bash
cub-netpol promote                                            # dry-run: what's behind upstream
cub-netpol promote --where "Slug LIKE 'default-deny-%'" \
  --commit --change-desc "Promote default-deny baseline to downstreams. User prompt: ..."
```

Scope to the policies you mean with `--where`.

## The loop

1. **Preview** with no `--commit` — `fleet default-deny` lists the namespaces it would protect; `promote` lists the Units behind upstream.
2. **Confirm** the scope (count, clusters, namespaces) with the user — fleet ops have wide blast radius.
3. **Commit** with a `--change-desc` that reads sensibly per Unit (the same description is recorded on every affected Unit).
4. **Stop.** Units are created/upgraded, NOT applied. Roll out via **cub-apply**. For a large promotion, consider wrapping it in a ChangeSet (see **promote-release**).

## Stop conditions

- The user hasn't confirmed a wide blast radius — show the dry-run and confirm before `--commit`.
- A `promote` upgrade reports merge conflicts — resolve via **cub-mutate**, don't force.
- Apply/rollout requested — hand off to **cub-apply**.

## Tool boundary

Allowed: the dry-run/commit fleet writes above (with `--change-desc`) plus read commands. Not allowed: applying to clusters, `kubectl` mutation, bypassing gates.

## References

- `cub-netpol fleet default-deny --help`, `cub-netpol promote --help`.
- Companion skills: **netpol-audit**, **netpol-fix**, **promote-release**, **cub-apply**.
