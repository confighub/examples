---
name: rbac-fleet
description: 'Apply RBAC changes across many ConfigHub Units at once with the cub-rbac CLI: bulk structured edits (fleet-edit) and override-preserving variant propagation from upstream (promote). Use for "add deletecollection to the developer role in every dev cluster", "remove the wildcard verb from this persona role fleet-wide", "add the oncall group to viewers across prod", "propagate the base RBAC change to all staging clones", "upgrade downstream RBAC units to their upstream", "which downstream units are behind their base?". Both are dry-run by default and require --commit + --change-desc; they never bypass ApplyGates and never apply to clusters. Not for a single Unit (use rbac-edit), not for inventory/queries (use rbac-audit / rbac-whocan), not for installing policy (use rbac-guardrails), not for rolling out to clusters (use cub-apply).'
phase: act
allowed-tools: Bash(cub-rbac --help) Bash(cub-rbac * --help) Bash(cub auth status) Bash(cub-rbac preflight) Bash(cub-rbac snapshot *) Bash(cub-rbac list *) Bash(cub-rbac fleet-edit *) Bash(cub-rbac promote *)
---

# rbac-fleet

Change RBAC across many Units in one server-side request. Two operations:

- **fleet-edit** — apply the same structured edit (add/remove a verb or subject) to every Unit matching a `--where` selector, e.g. a persona role replicated across clusters.
- **promote** — upgrade downstream (cloned) Units to their upstream's latest revision, preserving intentional local overrides — propagating a base RBAC change to its per-environment variants.

Both are **dry-run by default** and write nothing until you re-run with `--commit` and a `--change-desc`. Neither applies to clusters.

## Why this matters

The fleet is queried and changed like a database: one `--where` selector compiles to a single server-side operation — no looping over clusters, no drift between them. fleet-edit makes the same change everywhere it matches; promote moves variants forward from their base while keeping each environment's deliberate differences.

## When to use

- "Add `deletecollection` to the `developer` ClusterRole in every dev cluster" → `fleet-edit add-verb`.
- "Remove the wildcard verb from this persona role fleet-wide" → `fleet-edit remove-verb`.
- "Add the `oncall` group to `viewers` across prod" → `fleet-edit add-subject`.
- "Propagate the base RBAC change to all staging clones" / "upgrade downstream units to upstream" → `promote`.
- "Which downstream units are behind their base?" → `promote` dry-run (lists what would change).

## Do not load for

- A change to a single Unit — use **rbac-edit**.
- Inventory / who-can / findings — use **rbac-audit** / **rbac-whocan** / **rbac-findings**.
- Installing or enforcing policy guardrails — use **rbac-guardrails**.
- Rolling the changes out to clusters — use `cub unit apply` / the **cub-apply** skill.

## Preflight gates

1. `cub-rbac preflight` succeeds (cub installed, ConfigHub session valid). If not, ask the user to run `cub auth login` and retry.
2. The user has Edit permission on the targeted Units (the commit fails server-side otherwise — report it, don't retry blindly).

## Scoping — `--where` is required

Both commands require `--where`, a ConfigHub filter selecting the Units to change. It is ANDed with `ToolchainType = 'Kubernetes/YAML'`. The grammar is **AND-only** (no OR, no parentheses). Common selectors:

- `--where "Space.Labels.Environment = 'dev'"` — by standard Space label (Units inherit their cluster's Space labels).
- `--where "Space.Labels.Environment = 'prod' AND Space.Labels.Region = 'us-east'"`
- `--where "Slug = 'rbac'"` — by Unit slug across the fleet.

Confirm the selector hits the intended Units first with **rbac-audit** (`cub-rbac snapshot` / `list`).

## The loop

1. **Scope & preview (dry-run).** Run with no `--commit`; cub-rbac reports the Units that would change and writes nothing:
   ```bash
   cub-rbac fleet-edit add-verb --where "Space.Labels.Environment = 'dev'" \
     --role-kind ClusterRole --role developer --rule 0 --verb deletecollection
   cub-rbac promote --where "Space.Labels.Environment = 'staging'"
   ```
   For fleet-edit, "would change N Units" lists each `space/unit`. For promote, it lists the downstream Units behind their upstream (only Units with `UpstreamRevisionNum > 0` are considered; ones already at head are no-ops).
2. **Review the blast radius** with the user — fleet operations touch many Units. If the list is wrong, fix the `--where`; never commit an over-broad change.
3. **Commit** with a real change description capturing the request and clarifications:
   ```bash
   cub-rbac fleet-edit add-verb --where "Space.Labels.Environment = 'dev'" \
     --role-kind ClusterRole --role developer --rule 0 --verb deletecollection \
     --commit --change-desc "dev developers: allow deletecollection (OPS-12)"
   cub-rbac promote --where "Space.Labels.Environment = 'prod'" \
     --commit --change-desc "propagate base RBAC update to prod variants (OPS-12)"
   ```
4. **Stop.** The changes created new revisions; they are NOT applied. Hand off rollout to **cub-apply** (`cub unit apply`), which respects ApplyGates.

## Flags

- `fleet-edit <add-verb|remove-verb|add-subject|remove-subject>`: same edit flags as `edit` (`--role-kind`/`--role`/`--rule`/`--verb` or `--binding-kind`/`--binding`/`--subject-kind`/`--subject-name`/`--subject-namespace`), plus `--where`, `--commit`, `--change-desc`.
- `promote`: `--where`, `--commit`, `--change-desc`.

## Tool boundary

- Allowed: `cub-rbac fleet-edit` / `promote` (dry-run + commit), read-only inspection (`cub-rbac snapshot/list`).
- Not allowed: bypassing gates, applying to clusters, deleting Units, raw `kubectl`.

## Stop conditions

- The dry-run blast radius is wrong or larger than intended — narrow `--where`, never commit.
- A commit fails on permission or a gate — report the server message; do not bypass.
- The user wants the changes live — hand off to **cub-apply**.

## Safety

Fleet operations are high blast-radius. Be especially careful with edits touching `cluster-admin`, wildcards, or privilege-escalation verbs across many clusters, and always confirm the dry-run Unit list before `--commit`.

## References

- `cub-rbac fleet-edit --help` (and per-subcommand), `cub-rbac promote --help`.
- Companion skills: **rbac-edit** (single Unit), **rbac-audit** (scope check), **promote-release** (general promotion mechanics), **cub-apply** (rollout).
