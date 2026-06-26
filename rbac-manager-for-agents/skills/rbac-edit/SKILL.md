---
name: rbac-edit
description: 'Make guardrailed structured edits to a single Kubernetes RBAC Unit in ConfigHub with the cub-rbac CLI: add/remove a verb on a role rule, or add/remove a subject on a binding. Use for "give the viewer role get on pods", "remove the wildcard verb from this role", "add the oncall group to the admins binding", "revoke alice from breakglass", "drop the deletecollection verb". Edits are dry-run by default and require an explicit --commit with a --change-desc; they never bypass ApplyGates and never apply to the cluster. Not for inventory/queries (use rbac-audit / rbac-whocan), not for installing policy (use rbac-guardrails), not for applying/rolling out a change (use cub unit apply / the cub-apply skill).'
phase: act
allowed-tools: Bash(cub-rbac --help) Bash(cub-rbac * --help) Bash(cub auth status) Bash(cub-rbac preflight) Bash(cub-rbac snapshot *) Bash(cub-rbac list *) Bash(cub unit data *) Bash(cub unit get *) Bash(cub-rbac edit *)
---

# rbac-edit

Apply a single structured change to one RBAC Unit — add/remove a verb on a role rule, or add/remove a subject on a binding. The change is applied by a shared, parameterized server-side `set-yq` Invocation that modifies the literal YAML in place (comments and formatting preserved); the CLI supplies only the variable values as parameters. It is **dry-run by default**; nothing is written until you re-run with `--commit` and a `--change-desc`.

## Why this matters

This is the safe write path for RBAC: structured edits instead of hand-editing YAML, a diff preview before any write, a recorded change description for provenance, and no gate-bypassing. The edit only changes ConfigHub's stored config (a new revision); rolling it out to a cluster is a separate, deliberate step.

## When to use

- "Give the `viewer` ClusterRole `get` on pods" → `add-verb`.
- "Remove the wildcard verb / drop `deletecollection` from this role" → `remove-verb`.
- "Add the `oncall` group to the `admins` binding" → `add-subject`.
- "Revoke `alice` / remove the `ci` service account from `breakglass`" → `remove-subject`.

## Do not load for

- Finding/inspecting RBAC — use **rbac-audit** (`list`) or **rbac-whocan**.
- Installing/enforcing policy guardrails — use **rbac-guardrails**.
- Applying or rolling out the change to a cluster — that's `cub unit apply` (the **cub-apply** skill); this skill only edits stored config.
- Whole-Unit rewrites or non-RBAC fields — use `cub`/cub-mutate directly.

## Preflight gates

1. `cub-rbac preflight` succeeds (cub installed, ConfigHub session valid). If not, ask the user to run `cub auth login` and retry.
2. The shared edit Invocations exist. They are created once per organization with `cub-rbac edit install` (the same way the guardrail Triggers are installed). If an edit fails because the Invocation is not found, run `cub-rbac edit install` and retry.
3. The user has Edit permission on the target Unit (the commit will fail server-side otherwise — report the error, don't retry blindly).

## The loop

1. **Locate the resource.** Find the Unit (`<space>/<unit>`) and the exact role/binding name with **rbac-audit** (`cub-rbac list --kind ... --cluster ...`). For a verb edit you also need the **rule index** within the role — inspect the Unit's YAML to find it:
   ```bash
   cub unit data <unit> --space <space>      # read the role's rules[] to pick the index
   ```
2. **Preview (dry-run).** Run the edit with no `--commit`; cub-rbac prints the exact mutation diff and writes nothing:
   ```bash
   cub-rbac edit add-verb <space>/<unit> --role-kind ClusterRole --role viewer --rule 0 --verb get
   cub-rbac edit remove-verb <space>/<unit> --role-kind ClusterRole --role admin --rule 0 --verb '*'
   cub-rbac edit add-subject <space>/<unit> --binding-kind ClusterRoleBinding --binding viewers --subject-kind Group --subject-name oncall
   cub-rbac edit remove-subject <space>/<unit> --binding-kind RoleBinding --binding rb --subject-kind ServiceAccount --subject-name ci --subject-namespace apps
   ```
3. **Confirm the diff** with the user (or against the stated intent). If it's not what was asked, adjust the flags — don't commit a wrong diff.
4. **Commit** with a real change description that captures the user's request and any clarifications:
   ```bash
   cub-rbac edit add-verb <space>/<unit> --role-kind ClusterRole --role viewer --rule 0 --verb get \
     --commit --change-desc "grant viewer get on pods (ticket OPS-12)"
   ```
5. **Stop.** The edit created a new revision; it is NOT applied. If the user wants it live, hand off to **cub-apply** (`cub unit apply`), which respects ApplyGates.

## Flags & rules

- `add-verb` / `remove-verb`: `--role-kind` (Role|ClusterRole), `--role`, `--rule` (index), `--verb`. add-verb is idempotent (no duplicates).
- `add-subject` / `remove-subject`: `--binding-kind` (RoleBinding|ClusterRoleBinding), `--binding`, `--subject-kind` (User|Group|ServiceAccount), `--subject-name`, and `--subject-namespace` (required for ServiceAccount).
- `--commit` performs the write; without it you get a dry-run. `--change-desc` is required with `--commit`.

## Tool boundary

- Allowed: `cub-rbac edit` (dry-run + commit), read-only inspection (`cub-rbac list/snapshot`, `cub unit data/get`).
- Not allowed: bypassing gates, applying to clusters, deleting Units, raw `kubectl`/hand-edited YAML.

## Stop conditions

- The dry-run diff doesn't match the intent — fix flags, never commit a wrong change.
- Committing fails on permission or a gate — report the server message; do not try to bypass it.
- The user wants the change live — hand off to **cub-apply**.

## Safety

RBAC edits are high-stakes. Be especially careful with edits that touch `cluster-admin`, wildcard verbs/resources, or privilege-escalation verbs (escalate/bind/impersonate), and with removing a subject that could lock the operator out. Surface these in the confirmation step.

## References

- `cub-rbac edit --help` and per-subcommand `--help`.
- Companion skills: **rbac-audit**, **rbac-whocan**, **rbac-findings**, **rbac-guardrails**, and **cub-apply** (to roll out).
