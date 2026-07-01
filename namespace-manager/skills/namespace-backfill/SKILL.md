---
name: namespace-backfill
description: 'Fix namespace-envelope gaps as config-as-data with the cub-namespace CLI — stamp pod-security defaults on a Namespace Unit (apply-envelope) and clone a base envelope''s default-deny NetworkPolicy + baseline RBAC into an existing Space, re-homed with set-namespace (backfill). Use for "add pod-security labels to namespace X", "backfill the default-deny / RBAC into apptique-prod", "bring this namespace up to the envelope standard", "fix the missing-pod-security / missing-default-deny findings". Dry-run by default; requires --commit --change-desc. Not for read-only checks (use namespace-audit / namespace-findings) or enforcement Triggers (a later skill).'
phase: act
allowed-tools: Bash(cub-namespace --help) Bash(cub-namespace * --help) Bash(cub auth status) Bash(cub-namespace preflight) Bash(cub-namespace envelope *) Bash(cub-namespace findings *) Bash(cub-namespace apply-envelope *) Bash(cub-namespace backfill *)
---

# namespace-backfill

Fix the gaps `namespace-findings` / `namespace-audit` surface — **as data**, so the cluster converges through the normal apply pipeline with no drift. Two commands:

- **`apply-envelope`** — runs `set-pod-security-defaults` on a Space's Namespace Unit(s) (the fix for `missing-pod-security`).
- **`backfill`** — clones a base envelope's default-deny NetworkPolicy + baseline RBAC into an existing Space and re-homes them with `set-namespace` (the fix for `missing-default-deny` / `missing-baseline-rbac`).

Both **create/edit Units but do not apply them** to a cluster — rolling out is a separate, deliberate `cub unit apply`.

## Why this matters

A runtime tenancy controller injects policy objects into live namespaces; correcting the template doesn't reconcile what's already there, and ad-hoc `kubectl` fixes drift from the source of record. `cub-namespace` fixes the **data**: `apply-envelope` uses a hermetic, idempotent function; `backfill` clones from a base envelope Space (or the installer-uploaded `namespace-envelope` package) via the bulk-clone API. Both are **dry-run by default** and require `--commit --change-desc`.

## When to use

- "Add pod-security labels to namespace X." / "fix the missing-pod-security findings." → `apply-envelope --space <s>`.
- "Backfill the default-deny and baseline RBAC into apptique-prod." → `backfill --space <s> --from <base> --namespace <ns>`.
- "Bring this namespace up to the envelope standard."

## Do not load for

- Read-only checks — use **namespace-audit** / **namespace-findings**.
- Cross-variant consistency — use **namespace-consistency**.
- Enforcement Triggers / guardrails — a later skill.
- Applying Units to a cluster — that is `cub unit apply` (a separate, deliberate step).

## Preflight gates

1. `cub-namespace preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. The user has write permission on the target Space(s).

## The loop

1. **See the gap** (read-only): `cub-namespace envelope --incomplete-only` or `cub-namespace findings --severity high`.
2. **Preview** the fix (dry-run — the default):
   ```bash
   cub-namespace apply-envelope --space apptique-prod
   cub-namespace backfill --space apptique-prod --from namespace-envelope --namespace apptique
   ```
   `apply-envelope` reports which Namespace Units *would* change (a no-op where pod-security is already present). `backfill` lists the base Units it would clone — it **excludes the base's Namespace when the dest already has one** (cloning it would create a `duplicate-namespace` collision), so it adds only the missing NetworkPolicy + RBAC members.
3. **Commit** with `--commit --change-desc` (compose the description: summary, verbatim user prompt, condensed clarifications):
   ```bash
   cub-namespace apply-envelope --space apptique-prod --commit \
     --change-desc "Stamp pod-security defaults. User prompt: ..."
   cub-namespace backfill --space apptique-prod --from namespace-envelope --namespace apptique --commit \
     --change-desc "Backfill default-deny + baseline RBAC. User prompt: ..."
   ```
   Re-runs are idempotent (`apply-envelope` no-ops; `backfill` clones with allow-exists).
4. **Verify**: `cub-namespace envelope --cluster <c> --namespace <ns>` shows the namespace complete; `cub-namespace findings --cluster <c>` shows the gaps cleared.
5. **Roll out** is a separate step — hand off to `cub-apply` (the Units are created/edited, not applied).

## Stop conditions

- An ApplyGate attaches (a validating Trigger failed). **Do not bypass** — diagnose and fix the data (or the Trigger), via **triggers-and-applygates**.
- `backfill` reports "nothing missing" — the envelope is already complete; nothing to do.
- The user wants to apply to a cluster — hand off to `cub-apply`.

## Tool boundary

Allowed: `apply-envelope`, `backfill` (dry-run by default; `--commit` passes `--change-desc`), and the read commands. Not allowed: bypassing gates, `kubectl` mutations, applying to clusters.

## References

- `cub-namespace apply-envelope --help`, `cub-namespace backfill --help`.
- Companion skills: **namespace-audit**, **namespace-findings**, `kubernetes-resources`, `triggers-and-applygates`, `cub-apply`.
