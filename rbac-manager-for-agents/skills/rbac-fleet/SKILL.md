---
name: rbac-fleet
description: 'Apply RBAC changes across many ConfigHub Units at once with the cub-rbac CLI: bulk structured edits (fleet-edit). Use for "add deletecollection to the developer role in every dev cluster", "remove the wildcard verb from this persona role fleet-wide", "add the oncall group to viewers across prod". Dry-run by default; requires --commit + --change-desc; never bypasses ApplyGates and never publishes a Release. Not for a single Unit (use rbac-edit), not for inventory/queries (use rbac-audit / rbac-whocan), not for installing policy (use rbac-guardrails), not for propagating a base change to its variants (use promote-release), not for rolling out to clusters (use release-publish).'
phase: act
allowed-tools: Bash(cub-rbac --help) Bash(cub-rbac * --help) Bash(cub auth status) Bash(cub-rbac preflight) Bash(cub-rbac snapshot *) Bash(cub-rbac list *) Bash(cub-rbac edit install) Bash(cub-rbac fleet-edit *)
---

# rbac-fleet

Change RBAC across many Units in one server-side request:

- **fleet-edit** — apply the same structured edit (add/remove a verb or subject) to every Unit matching a `--where` selector, e.g. a persona role replicated across clusters.

It is **dry-run by default** and writes nothing until you re-run with `--commit` and a `--change-desc`. It never publishes a Release.

## Why this matters

The fleet is queried and changed like a database: one `--where` selector compiles to a single server-side operation — no looping over clusters, no drift between them. fleet-edit makes the same change everywhere it matches.

## When to use

- "Add `deletecollection` to the `developer` ClusterRole in every dev cluster" → `fleet-edit add-verb`.
- "Remove the wildcard verb from this persona role fleet-wide" → `fleet-edit remove-verb`.
- "Add the `oncall` group to `viewers` across prod" → `fleet-edit add-subject`.

## Do not load for

- A change to a single Unit — use **rbac-edit**.
- Inventory / who-can / findings — use **rbac-audit** / **rbac-whocan** / **rbac-findings**.
- Installing or enforcing policy guardrails — use **rbac-guardrails**.
- Propagating a base change to its variant Spaces ("upgrade the downstreams", "which are behind their base?") — use **promote-release**.
- Rolling the changes out to clusters — use `cub release publish` / the **release-publish** skill.

## Preflight gates

1. `cub-rbac preflight` succeeds (cub installed, ConfigHub session valid). If not, ask the user to run `cub auth login` and retry.
2. The shared edit Invocations exist (used by `fleet-edit`). They are created once per organization with `cub-rbac edit install`. If a fleet-edit fails because the Invocation is not found, run `cub-rbac edit install` and retry.
3. The user has Edit permission on the targeted Units (the commit fails server-side otherwise — report it, don't retry blindly).

## Scoping — a selector is required

Both commands require a selector — either a raw `--where` filter or at least one label shorthand — so a fleet mutation is always deliberate and scoped. The selector is ANDed with `ToolchainType = 'Kubernetes/YAML'`. The grammar is **AND-only** (no OR, no parentheses — a parenthesized clause fails with `invalid attribute name`). Common selectors:

- `--environment dev` — label shorthand; expands to `Space.Labels.Environment = 'dev'`. Other shorthands: `--component`, `--region`, `--owner`, `--layer`, `--variant`.
- `--where "Space.Labels.Environment = 'dev'"` — the raw equivalent (Units inherit their cluster's Space labels).
- `--environment prod --region us-east` — shorthands AND together (and AND onto any raw `--where`).
- `--where "Target.ProviderType = 'OCI'"` — by any Unit/Space/Target attribute.
- `--where "Slug = 'rbac'"` — by Unit slug across the fleet.

Running with no selector at all is rejected.

Confirm the selector hits the intended Units first with **rbac-audit** (`cub-rbac snapshot` / `list`).

## The loop

1. **Scope & preview (dry-run).** Run with no `--commit`; cub-rbac reports the Units that would change and writes nothing:
   ```bash
   cub-rbac fleet-edit add-verb --where "Space.Labels.Environment = 'dev'" \
     --role-kind ClusterRole --role developer --rule 0 --verb deletecollection
   ```
   "would change N Units" lists each `space/unit`.
2. **Review the blast radius** with the user — fleet operations touch many Units. If the list is wrong, fix the `--where`; never commit an over-broad change.
3. **Commit** with a real change description capturing the request and clarifications:
   ```bash
   cub-rbac fleet-edit add-verb --where "Space.Labels.Environment = 'dev'" \
     --role-kind ClusterRole --role developer --rule 0 --verb deletecollection \
     --commit --change-desc "dev developers: allow deletecollection (OPS-12)"
   ```
4. **Stop.** The changes created new revisions; they are NOT published. Hand off rollout to **release-publish** (`cub release publish`), which respects ApplyGates.

## Flags

- `fleet-edit <add-verb|remove-verb|add-subject|remove-subject>`: same edit flags as `edit` (`--role-kind`/`--role`/`--rule`/`--verb` or `--binding-kind`/`--binding`/`--subject-kind`/`--subject-name`/`--subject-namespace`), plus `--where`, `--commit`, `--change-desc`.

## Tool boundary

- Allowed: `cub-rbac fleet-edit` (dry-run + commit), read-only inspection (`cub-rbac snapshot/list`).
- Not allowed: bypassing gates, publishing a Release, deleting Units, raw `kubectl`.

## Stop conditions

- The dry-run blast radius is wrong or larger than intended — narrow `--where`, never commit.
- A commit fails on permission or a gate — report the server message; do not bypass.
- Variant propagation requested — hand off to **promote-release**.
- The user wants the changes live — hand off to **release-publish**.

## Safety

Fleet operations are high blast-radius. Be especially careful with edits touching `cluster-admin`, wildcards, or privilege-escalation verbs across many clusters, and always confirm the dry-run Unit list before `--commit`.

## References

- `cub-rbac fleet-edit --help` (and per-subcommand).
- Companion skills: **rbac-edit** (single Unit), **rbac-audit** (scope check), **promote-release** (variant propagation), **release-publish** (rollout).
