---
name: netpol-guardrails
description: 'Install, inspect, and feed the NetworkPolicy guardrail policy pack in ConfigHub with the cub-netpol CLI — making NetworkPolicy findings enforced (advisory ApplyWarnings) instead of just reported. Use for "enforce our network-policy rules", "block allow-all policies", "warn on egress to 0.0.0.0/0", "set up NetworkPolicy guardrails", "which Units have NetworkPolicy warnings?", "annotate the uncovered Units so they get flagged". Installs Warn=true vet-celexpr Triggers + a Filter (annotate-then-validate for coverage); dry-run by default. Not for one-off checks (use netpol-findings) or fixing config (use netpol-fix).'
phase: act
allowed-tools: Bash(cub-netpol --help) Bash(cub-netpol * --help) Bash(cub auth status) Bash(cub-netpol preflight) Bash(cub-netpol findings *) Bash(cub-netpol guardrails install *) Bash(cub-netpol guardrails status *) Bash(cub-netpol guardrails annotate *)
---

# netpol-guardrails

Make NetworkPolicy findings **enforced**, not advisory. Installs a pack of validation policies (defined once in a policy Space, enforced fleet-wide via a shared Filter) and the annotate-then-validate loop that turns a coverage finding into an ApplyWarning.

## Why this matters

The manager itself cannot set ApplyWarnings/ApplyGates — only a failed validating **Trigger** can. So enforcement is two-sided: per-Unit shape rules are pure `vet-celexpr` Triggers; the set-aware coverage check uses **annotate-then-validate** — `guardrails annotate` writes a finding annotation onto flagged Units, and an installed Trigger reads it and raises a warning. Triggers are created with `Warn=true` (advisory ApplyWarnings, never blocking), so installing on an existing fleet blocks no one.

## The pack

| Trigger | Checks |
| --- | --- |
| `netpol-no-allow-all-ingress` | ingress rules must name their sources (no empty `from`) |
| `netpol-no-wide-cidr-egress` | no egress to `0.0.0.0/0` |
| `netpol-coverage-finding` | warns while a `netpol.confighub.com/finding` annotation is present |

## When to use

- "Enforce our NetworkPolicy rules." / "set up NetworkPolicy guardrails."
- "Block / warn on allow-all policies / egress to 0.0.0.0/0."
- "Which Units have NetworkPolicy warnings or gates?" → `guardrails status`.
- "Annotate the uncovered Units so they're flagged." → `guardrails annotate`.

## Do not load for

- A one-off hygiene scan (no enforcement) — use **netpol-findings**.
- Fixing the config the guardrails flag — use **netpol-fix** / **netpol-fleet**.
- General Trigger/ApplyGate setup beyond this pack — use **triggers-and-applygates**.

## Preflight gates

1. `cub-netpol preflight` succeeds. If not, ask the user to run `cub auth login` and retry.
2. The user has permission on the policy Space and the Spaces being wired.

## The loop

1. **Preview** the install (dry-run by default): it prints the policy Space, the Triggers, the Filter, and which Spaces it would wire (and which it would skip).
   ```bash
   cub-netpol guardrails install --where-space "Labels.Component = 'rbac'" -o table
   ```
2. **Install** the pack with `--commit`:
   ```bash
   cub-netpol guardrails install --where-space "Slug LIKE 'apptique-%'" --commit
   ```
   Spaces with a pre-existing custom `WhereTrigger` are skipped (not clobbered) and reported; wire those deliberately with `cub space update <space> --trigger-filter netpol-policy/netpol-guardrails --where-trigger "-"` (see **triggers-and-applygates**).
3. **Annotate** coverage findings so the coverage Trigger flags them (dry-run by default):
   ```bash
   cub-netpol guardrails annotate --commit --change-desc "Flag uncovered Units. User prompt: ..."
   ```
   Re-run after fixing config — a clean Unit produces no finding, so its annotation is cleared.
4. **Inspect** what's flagged:
   ```bash
   cub-netpol guardrails status -o table
   ```
5. **Promote a rule to blocking** later (the manager never does this silently): `cub trigger update <slug> --space netpol-policy --unwarn`.

## Stop conditions

- A Space already selects Triggers another way — report it (the install skips it); wire it deliberately, don't clobber existing policy config.
- The user wants to *fix* a flagged Unit — hand off to **netpol-fix**.
- Never bypass a gate by dropping a Trigger or editing gate state — fix the data, or fix the Trigger in the policy Space.

## Tool boundary

Allowed: `guardrails install/status/annotate` and read commands; `annotate` passes `--change-desc`. Not allowed: bypassing gates, `kubectl`, applying to clusters.

## References

- `cub-netpol guardrails install --help`, `... status --help`, `... annotate --help`.
- Companion skills: **netpol-findings**, **netpol-fix**, **triggers-and-applygates**.
