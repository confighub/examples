---
name: workload-harden
description: 'Fix a single Kubernetes workload''s production-readiness gaps as config-as-data with the cub-workload CLI — harden (security-context + automount defaults), set-resources (requests/limits by tier or explicit), set-probes (liveness/readiness/startup), ensure-pdb (author a PodDisruptionBudget whose selector is derived from the workload), ensure-spread (pod anti-affinity or topology spread). Use for "harden the checkout deployment", "set resources on workload X", "add probes", "add a PDB for this workload", "spread this deployment across nodes", "fix workload X''s findings". Dry-run by default; requires --commit --change-desc. Not for read-only checks (use workload-audit / workload-findings), bulk edits across many workloads or the profile library (use workload-fleet), or enforcement Triggers (use workload-guardrails).'
phase: act
allowed-tools: Bash(cub-workload --help) Bash(cub-workload * --help) Bash(cub auth status) Bash(cub-workload preflight) Bash(cub-workload readiness *) Bash(cub-workload availability *) Bash(cub-workload findings *) Bash(cub-workload harden *) Bash(cub-workload set-resources *) Bash(cub-workload set-probes *) Bash(cub-workload ensure-pdb *) Bash(cub-workload ensure-spread *)
---

# workload-harden

Fix the gaps `workload-findings` / `workload-audit` / `workload-availability` surface on **one workload** — as data, so the cluster converges through the normal apply pipeline with no drift. Five commands, all targeting a single `<space>/<unit>`:

- **`harden`** — `set-pod-container-security-context-defaults` + `set-automount-service-account-token-false` (fixes security findings).
- **`set-resources`** — `set-container-resources` by `--tier small|medium|large` or explicit `--cpu/--memory/--limit-factor` (fixes missing requests/limits).
- **`set-probes`** — `set-container-probe-defaults` (fixes missing probes).
- **`ensure-pdb`** — derives the PDB selector from the workload's pod labels and authors a new PDB Unit (fixes uncovered availability).
- **`ensure-spread`** — adds pod anti-affinity (`--anti-affinity soft|hard`) or a topology spread constraint (fixes no-spread).

All **create/edit Units but do not publish them** — rolling out is a separate, deliberate `cub release publish <space>`, which bundles the Space's Units for its release Target.

## Why this matters

Per-object validators return pass/fail and a human then `kubectl edit`s the cluster, drifting from the source of record. `cub-workload` fixes the **data** with hermetic, idempotent functions (or, for `ensure-pdb`, authors a new Unit whose selector matches the workload). Every command is **dry-run by default** and requires `--commit --change-desc`.

## When to use

- "Harden the checkout deployment." → `harden checkout-prod/checkout`.
- "Set medium resources on workload X." → `set-resources <space>/<unit> --tier medium` (or `--cpu/--memory`).
- "Add probes." → `set-probes <space>/<unit>`.
- "Add a PDB for this workload." → `ensure-pdb <space>/<unit>` (`--min-available` / `--max-unavailable`).
- "Spread this deployment across nodes/zones." → `ensure-spread <space>/<unit>` (`--anti-affinity soft` default; `--topology-spread`; `--topology-key`).

## Do not load for

- Read-only checks — use **workload-audit** / **workload-availability** / **workload-findings**.
- The same fix across *many* workloads, or the reusable profile library — use **workload-fleet**.
- Enforcement Triggers / guardrails — use **workload-guardrails**.
- Publishing the Space's Release — that is `cub release publish <space>` (a separate step).

## Preflight gates

1. `cub-workload preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. The user has write permission on the target Space.

## The loop

1. **See the gap** (read-only): `cub-workload readiness --failing-only` / `cub-workload availability --issues-only` / `cub-workload findings --severity high`.
2. **Preview** the fix (dry-run — the default), e.g.:
   ```bash
   cub-workload harden apptique-prod/frontend
   cub-workload set-resources apptique-prod/frontend --tier medium
   cub-workload ensure-pdb apptique-prod/frontend
   cub-workload ensure-spread apptique-prod/frontend            # soft anti-affinity
   ```
   Each reports whether the Unit *would* change (a no-op where the fix is already present).
3. **Commit** with `--commit --change-desc` (compose the description: summary, verbatim user prompt, condensed clarifications):
   ```bash
   cub-workload harden apptique-prod/frontend --commit \
     --change-desc "Apply security-context + automount defaults. User prompt: ..."
   ```
   Re-runs are idempotent.
4. **Verify**: `cub-workload readiness --cluster <c> --namespace <ns>` / `availability` shows the dimension now passes.
5. **Roll out** is a separate step — hand off to `release-publish` (Units are created/edited, not published).

## Safety

- Prefer `ensure-spread --anti-affinity soft`; `hard` (`requiredDuringScheduling`) can leave replicas `Pending` when the cluster has fewer eligible nodes than replicas.
- `harden` sets `readOnlyRootFilesystem: true`; if a container legitimately writes to its filesystem or needs the ServiceAccount token, record an exception rather than hardening it blindly.
- Warn before a `set-resources` whose limits are low enough to risk OOMKill/throttle on a running workload.

## Stop conditions

- An ApplyGate attaches (a validating Trigger failed). **Do not bypass** — diagnose and fix the data (or the Trigger), via **triggers-and-applygates**.
- The fix would apply to many workloads — hand off to **workload-fleet**.
- The user wants the change deployed — hand off to `release-publish`.

## Tool boundary

Allowed: `harden`, `set-resources`, `set-probes`, `ensure-pdb`, `ensure-spread` (dry-run by default; `--commit` passes `--change-desc`), and the read commands. Not allowed: bypassing gates, `kubectl` mutations, publishing a Release.

## References

- `cub-workload harden --help`, `… set-resources --help`, `… set-probes --help`, `… ensure-pdb --help`, `… ensure-spread --help`.
- Companion skills: **workload-audit**, **workload-availability**, **workload-findings**, **workload-fleet**, `kubernetes-resources`, `triggers-and-applygates`, `release-publish`.
