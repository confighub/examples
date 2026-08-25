---
name: netpol-fleet
description: 'Fleet-wide NetworkPolicy remediation in ConfigHub with the cub-netpol CLI: create a default-deny for every uncovered namespace at once. Use for "default-deny every namespace that lacks one", "remediate all our coverage gaps". Dry-run by default; requires --commit with --change-desc; Units are created but NOT published. Not for a single namespace/policy (use netpol-fix), inventory/findings (use netpol-audit/netpol-findings), installing enforcement (use netpol-guardrails), or propagating a baseline downstream (use promote-release).'
phase: act
allowed-tools: Bash(cub-netpol --help) Bash(cub-netpol * --help) Bash(cub auth status) Bash(cub-netpol preflight) Bash(cub-netpol coverage *) Bash(cub-netpol findings *) Bash(cub-netpol fleet default-deny *)
---

# netpol-fleet

Fleet-wide remediation. Bulk-close coverage gaps across every namespace that lacks a default-deny. **Dry-run by default**; nothing is written until `--commit` with a `--change-desc`, and Units are created but **not published**.

## Why this matters

`netpol-fix` closes one gap; this closes them all at once, driven directly by the coverage analysis — the fleet-scale version of the same config-as-data, no-drift fix.

## When to use

- "Default-deny every namespace that lacks one." / "remediate all our coverage gaps." → `fleet default-deny`.

## Do not load for

- A single namespace or policy — use **netpol-fix**.
- Inventory / findings — **netpol-audit**, **netpol-findings**.
- Installing enforcement Triggers — **netpol-guardrails**.
- Propagating a baseline from a base Space to its variants — **promote-release**.
- Publishing the Space's Release — **release-publish**.

## Preflight gates

1. `cub-netpol preflight` succeeds. If not, ask the user to run `cub auth login` and retry.

## The toolkit

### Bulk remediation — `cub-netpol fleet default-deny`

Finds every namespace with workloads but no default-deny ingress (the coverage gap) and authors a default-deny Unit for each, in its workloads' Space. Idempotent — covered namespaces are skipped.

```bash
cub-netpol fleet default-deny -o table                       # dry-run: what would be created
cub-netpol fleet default-deny --egress                        # also deny egress, allowing DNS
cub-netpol fleet default-deny --cluster prod-cluster \
  --commit --change-desc "Bulk default-deny for uncovered namespaces. User prompt: ..."
```

## The loop

1. **Preview** with no `--commit` — `fleet default-deny` lists the namespaces it would protect.
2. **Confirm** the scope (count, clusters, namespaces) with the user — fleet ops have wide blast radius.
3. **Commit** with a `--change-desc` that reads sensibly per Unit (the same description is recorded on every affected Unit).
4. **Stop.** Units are created, NOT published. Roll out via **release-publish**.

## Stop conditions

- The user hasn't confirmed a wide blast radius — show the dry-run and confirm before `--commit`.
- Propagating the baseline to downstream variants — hand off to **promote-release**.
- Publish/rollout requested — hand off to **release-publish**.

## Tool boundary

Allowed: the dry-run/commit fleet writes above (with `--change-desc`) plus read commands. Not allowed: publishing a Release, `kubectl` mutation, bypassing gates.

## References

- `cub-netpol fleet default-deny --help`.
- Companion skills: **netpol-audit**, **netpol-fix**, **promote-release**, **release-publish**.
