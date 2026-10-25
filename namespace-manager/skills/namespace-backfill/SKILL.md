---
name: namespace-backfill
description: 'Fix namespace-envelope gaps as config-as-data with the cub-namespace CLI — set each namespace''s Pod Security Standard to the most restrictive level its workloads meet, on the Namespace and its workloads (apply-envelope), and clone a base envelope Space''s Units into an existing Space, re-homed with set-namespace (backfill). Use for "add pod-security labels to namespace X", "backfill the base envelope into apptique-prod", "bring this namespace up to the envelope standard", "fix the missing-pod-security findings". Dry-run by default; requires --commit --change-desc. Not for read-only checks (use namespace-audit / namespace-findings) or enforcement Triggers (a later skill).'
phase: act
allowed-tools: Bash(cub-namespace --help) Bash(cub-namespace * --help) Bash(cub auth status) Bash(cub-namespace preflight) Bash(cub-namespace envelope *) Bash(cub-namespace findings *) Bash(cub-namespace pod-security *) Bash(cub-namespace apply-envelope *) Bash(cub-namespace backfill *)
---

# namespace-backfill

Fix the gaps `namespace-findings` / `namespace-audit` surface — **as data**, so the cluster converges through the normal apply pipeline with no drift. Two commands:

- **`apply-envelope`** — labels each selected namespace with the most restrictive Pod Security Standard level all its workloads meet (or `--level`), and copies the label onto the namespace's workloads so the `namespace-pod-security-standard` guardrail checks each against it. It never lowers a level: a namespace that already enforces a stricter one keeps it, and its workloads get copies, so the guardrail flags the ones it rejects. The fix for `missing-pod-security`.
- **`backfill`** — clones a base envelope Space's Units into an existing Space and re-homes them with `set-namespace`. What the base defines is the base's business; backfill clones and re-homes without reading the policies.

Both **create/edit Units but do not publish them** — rolling out is a separate, deliberate `cub release publish <space>`, which bundles the Space's Units for its release Target.

## Why this matters

A runtime tenancy controller injects policy objects into live namespaces; correcting the template doesn't reconcile what's already there, and ad-hoc `kubectl` fixes drift from the source of record. `cub-namespace` fixes the **data**: `apply-envelope` uses the hermetic, idempotent `set-label`, on the Namespace and on each workload's own metadata (never its pod template, so nothing restarts); `backfill` clones from a base envelope Space (or the installer-uploaded `namespace-envelope` package) via the bulk-clone API. Both are **dry-run by default** and require `--commit --change-desc`.

## When to use

- "Add pod-security labels to namespace X." / "fix the missing-pod-security findings." / "raise namespace X to restricted." → `apply-envelope --namespace <ns>` (or `--space <s>`, `--cluster <c>`).
- "Backfill the base envelope into apptique-prod." → `backfill --space <s> --from <base> --namespace <ns>`.
- "Bring this namespace up to the envelope standard."

## Do not load for

- Read-only checks — use **namespace-audit** / **namespace-findings**.
- Cross-variant consistency — use **namespace-consistency**.
- Enforcement Triggers / guardrails — a later skill.
- Publishing the Space's Release — that is `cub release publish <space>` (a separate, deliberate step).

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
   `apply-envelope` lists each selected namespace with the level it would apply (the ACHIEVABLE column of `cub-namespace pod-security`), `keep <level>` where the Namespace's own label stays, or why it is skipped (cluster-owned, unchecked workloads, no workloads and no `--level`, or workloads below `--level`), then each Unit's diff. To raise a level, run `cub-namespace pod-security` first, fix what blocks the next level (`cub-workload harden`, for one), and re-run `pod-security` to confirm before applying. `backfill` lists the base Units it would clone — it **excludes the base's Namespace when the dest already has one** (cloning it would create a `duplicate-namespace` collision), so it adds only the members the destination lacks.
3. **Commit** with `--commit --change-desc` (compose the description: summary, verbatim user prompt, condensed clarifications):
   ```bash
   cub-namespace apply-envelope --space apptique-prod --commit \
     --change-desc "Apply the Pod Security Standard level the workloads meet. User prompt: ..."
   cub-namespace backfill --space apptique-prod --from namespace-envelope --namespace apptique --commit \
     --change-desc "Backfill the base envelope. User prompt: ..."
   ```
   Re-runs are idempotent (`apply-envelope` no-ops; `backfill` clones with allow-exists).
4. **Verify**: `cub-namespace envelope --cluster <c> --namespace <ns>` shows the namespace complete; `cub-namespace findings --cluster <c>` shows the gaps cleared.
5. **Roll out** is a separate step — hand off to `release-publish` (the Units are created/edited, not published).

## Stop conditions

- A ValidationError attaches (a validating Trigger failed). **Do not bypass** — diagnose and fix the data (or the Trigger), via **triggers-and-applygates**.
- `backfill` reports "nothing missing" — the envelope is already complete; nothing to do.
- The user wants the change deployed — hand off to `release-publish`.

## Tool boundary

Allowed: `apply-envelope`, `backfill` (dry-run by default; `--commit` passes `--change-desc`), and the read commands. Not allowed: bypassing gates, `kubectl` mutations, publishing a Release.

## References

- `cub-namespace apply-envelope --help`, `cub-namespace backfill --help`.
- Companion skills: **namespace-audit**, **namespace-findings**, `kubernetes-resources`, `triggers-and-applygates`, `release-publish`.
