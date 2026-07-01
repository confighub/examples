---
name: namespace-enforce
description: 'Install, inspect, and feed the namespace-envelope guardrail pack in ConfigHub with the cub-namespace CLI — making envelope findings enforced (advisory ApplyWarnings) instead of just reported. Use for "enforce our namespace standards", "warn on namespaces without pod-security", "flag namespaces missing the envelope", "set up namespace guardrails", "which Units have namespace warnings?", "annotate the incomplete namespaces so they get flagged". Installs Warn=true vet-celexpr Triggers + a Filter (annotate-then-validate for envelope gaps); dry-run by default. Not for one-off checks (use namespace-findings) or fixing config (use namespace-backfill).'
phase: act
allowed-tools: Bash(cub-namespace --help) Bash(cub-namespace * --help) Bash(cub auth status) Bash(cub-namespace preflight) Bash(cub-namespace findings *) Bash(cub-namespace guardrails install *) Bash(cub-namespace guardrails status *) Bash(cub-namespace guardrails annotate *)
---

# namespace-enforce

Make namespace-envelope findings **enforced**, not advisory. Installs a pack of validation policies (defined once in a policy Space, enforced fleet-wide via a shared Filter) and the annotate-then-validate loop that turns a set-aware envelope finding into an ApplyWarning.

## Why this matters

The manager itself cannot set ApplyWarnings/ApplyGates — only a failed validating **Trigger** can. So enforcement is two-sided: per-Unit shape rules are pure `vet-celexpr` Triggers; the set-aware envelope checks use **annotate-then-validate** — `guardrails annotate` writes a finding annotation onto each incomplete namespace's Namespace Unit, and an installed Trigger reads it and raises a warning. Triggers are created with `Warn=true` (advisory ApplyWarnings, never blocking), so installing on an existing fleet blocks no one.

## The pack

| Trigger | Checks |
| --- | --- |
| `namespace-has-pod-security` | a `v1/Namespace` must carry a `pod-security.kubernetes.io/enforce` label |
| `namespace-envelope-finding` | warns while a `namespace.confighub.com/finding` annotation is present |

The **namespace-name invariant** (`metadata.namespace == normalizeName(Component)`) is enforced separately by a cluster-selected **mutating `set-namespace` Trigger** — active correction, the promotable option, wired outside this advisory pack.

## When to use

- "Enforce our namespace standards." / "set up namespace guardrails."
- "Warn on namespaces without pod-security labels."
- "Which Units have namespace warnings or gates?" → `guardrails status`.
- "Annotate the incomplete namespaces so they're flagged." → `guardrails annotate`.

## Do not load for

- A one-off scan (no enforcement) — use **namespace-findings**.
- Fixing the config the guardrails flag — use **namespace-backfill**.
- General Trigger/ApplyGate setup beyond this pack — use **triggers-and-applygates**.

## Preflight gates

1. `cub-namespace preflight` succeeds. If not, ask the user to run `cub auth login` and retry.
2. The user has permission on the policy Space and the Spaces being wired.

## The loop

1. **Preview** the install (dry-run by default): it prints the policy Space, the Triggers, the Filter, and which Spaces it would wire (and which it would skip).
   ```bash
   cub-namespace guardrails install --where-space "Slug LIKE 'apptique-%'" -o table
   ```
2. **Install** the pack with `--commit`:
   ```bash
   cub-namespace guardrails install --where-space "Slug LIKE 'apptique-%'" --commit
   ```
   Spaces with a pre-existing custom `WhereTrigger` or a different `TriggerFilterID` are skipped (not clobbered) and reported; wire those deliberately (see **triggers-and-applygates**). After wiring, existing Units re-evaluate on their next mutation — or force it now with `cub space update --patch <space> --refresh-triggers`.
3. **Annotate** envelope findings so the finding Trigger flags them (dry-run by default):
   ```bash
   cub-namespace guardrails annotate --commit --change-desc "Flag incomplete namespaces. User prompt: ..."
   ```
   Writes the finding annotation onto each incomplete namespace's Namespace Unit; the set-annotation mutation re-runs the Trigger, raising the warning. Re-run after fixing config.
4. **Inspect** what's flagged:
   ```bash
   cub-namespace guardrails status -o table
   ```
5. **Promote a rule to blocking** later (the manager never does this silently): `cub trigger update <slug> --space namespace-policy --unwarn`.

## Stop conditions

- A Space already selects Triggers another way — report it (the install skips it); wire it deliberately, don't clobber existing policy config.
- The user wants to *fix* a flagged namespace — hand off to **namespace-backfill**.
- Never bypass a gate by dropping a Trigger or editing gate state — fix the data, or fix the Trigger in the policy Space.

## Tool boundary

Allowed: `guardrails install/status/annotate` and read commands; `annotate` passes `--change-desc`. Not allowed: bypassing gates, `kubectl`, applying to clusters.

## References

- `cub-namespace guardrails install --help`, `... status --help`, `... annotate --help`.
- Companion skills: **namespace-findings**, **namespace-backfill**, **triggers-and-applygates**.
