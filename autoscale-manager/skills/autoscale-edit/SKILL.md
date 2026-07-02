---
name: autoscale-edit
description: 'Set Kubernetes autoscaling as config-as-data with the cub-autoscale CLI — edit a HorizontalPodAutoscaler''s min/max replicas and cpu/memory targets, convert an HPA to a KEDA ScaledObject, apply reusable autoscaling profiles to one Unit or across a whole selector, and promote changes downstream. Use for "raise the HPA max to 20", "set cpu target to 60%", "convert this HPA to KEDA", "make every prod HPA scale out earlier", "apply the hpa-range profile", "roll the autoscaling change downstream". Dry-run by default; requires --commit --change-desc. Not for read-only checks (use autoscale-audit / autoscale-findings) or enforcement Triggers (use autoscale-guardrails).'
phase: act
allowed-tools: Bash(cub-autoscale --help) Bash(cub-autoscale * --help) Bash(cub auth status) Bash(cub-autoscale preflight) Bash(cub-autoscale list *) Bash(cub-autoscale findings *) Bash(cub-autoscale set-hpa *) Bash(cub-autoscale convert-keda *) Bash(cub-autoscale profile) Bash(cub-autoscale profile *) Bash(cub-autoscale fleet-edit *) Bash(cub-autoscale promote *)
---

# autoscale-edit

Set how workloads autoscale — as data, dry-run by default. One HPA, an HPA→KEDA conversion, a profile across a `--where` selector, or promoted downstream.

- **`set-hpa <space>/<unit> [--min N] [--max N] [--cpu PCT] [--memory PCT]`** — edit an HPA's replica bounds and/or cpu/memory utilization targets.
- **`convert-keda <space>/<unit>`** — rewrite an HPA as an equivalent KEDA ScaledObject (preserves min/max and cpu/memory metrics). Runs the `convert-hpa-to-keda` function in an **embedded executor in-process** — no server-side function, no worker.
- **`profile install | list | apply`** — the autoscaling profile library (stored Invocations): `hpa-conservative` (cpu 60%), `hpa-aggressive` (cpu 85%), and parameterized `hpa-range` (`--param min=… --param max=…`).
- **`fleet-edit --profile <slug> [--where …]`** — apply a profile across a selector of autoscaler Units in one operation.
- **`promote`** — override-preserving upgrade of downstream Units.

All **edit Units but do not apply them** to a cluster — rolling out is a separate `cub unit apply`.

## Why this matters

`cub-autoscale` edits the source of record with `set-yq` under the hood (and the embedded executor for `convert-keda`); everything is **dry-run by default** and requires `--commit --change-desc`, and never bypasses ApplyGates. A pinned autoscaler (`min == max`) can't scale — widen the bounds. KEDA ScaledObjects also need the KEDA operator installed in the target cluster before they'll do anything.

## When to use

- "Raise web's HPA to 3–20 and cpu 60%." → `set-hpa web-prod/web --min 3 --max 20 --cpu 60`.
- "Convert this HPA to KEDA." → `convert-keda <space>/<unit>`.
- "Set every prod HPA to scale out earlier." → `fleet-edit --profile hpa-conservative --environment prod`.
- "Set explicit bounds via a profile." → `profile apply hpa-range <space>/<unit> --param min=3 --param max=15`.
- "Roll the autoscaling change downstream." → `promote --component <c>`.

## Do not load for

- Read-only checks — use **autoscale-audit** / **autoscale-findings**.
- Enforcement Triggers (a pinned-autoscaler warning, schema validation) — use **autoscale-guardrails**.
- Applying Units to a cluster — that is `cub unit apply`.

## Preflight gates

1. `cub-autoscale preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. For `profile apply` / `fleet-edit`, the library exists — run `cub-autoscale profile install` once if `profile list` is empty.
3. The user has write permission on the target Space(s).

## The loop

1. **See the gap**: `cub-autoscale findings` / `cub-autoscale list`.
2. **Preview** (dry-run — the default):
   ```bash
   cub-autoscale set-hpa web-prod/web --min 3 --max 20 --cpu 60
   cub-autoscale convert-keda web-prod/web
   cub-autoscale fleet-edit --profile hpa-conservative --environment prod
   ```
3. **Commit** with `--commit --change-desc` (summary + verbatim user prompt).
4. **Verify**: `cub-autoscale list --where "Slug = '<unit>'"` shows the new bounds/kind; `findings` clears.
5. **Roll out** is a separate step — hand off to `cub-apply`.

## Safety

- Don't pin an autoscaler: keep `min < max`. `set-hpa` rejects `--min > --max`.
- `convert-keda` only translates cpu/memory Resource metrics; Pods/Object/External metrics aren't converted (KEDA needs a matching scaler) — review the dry-run output.
- Scope `fleet-edit` narrowly (dry-run shows the count) before a fleet-wide commit.

## Stop conditions

- An ApplyGate attaches (a validating Trigger failed). **Do not bypass** — fix the data (or the rule), via **triggers-and-applygates**.
- The user wants to apply to a cluster — hand off to `cub-apply`.

## Tool boundary

Allowed: `set-hpa`, `convert-keda`, `profile`, `fleet-edit`, `promote` (dry-run by default; `--commit` passes `--change-desc`), and the read commands. Not allowed: bypassing gates, `kubectl` mutations, applying to clusters.

## References

- `cub-autoscale set-hpa --help`, `… convert-keda --help`, `… profile --help`, `… fleet-edit --help`, `… promote --help`.
- Companion skills: **autoscale-audit**, **autoscale-findings**, **autoscale-guardrails**, `promote-release`, `triggers-and-applygates`, `cub-apply`.
