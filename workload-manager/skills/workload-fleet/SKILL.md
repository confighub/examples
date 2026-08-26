---
name: workload-fleet
description: 'Fleet-scale workload remediation and the reusable profile library with the cub-workload CLI — profile install/list/apply (parameterized Invocations: resource tiers, harden, probes, anti-affinity, termination policy), and fleet-edit (apply a profile across a --where selector of workloads in one operation). Use for "harden every prod workload", "set medium resources across the checkout component", "apply the anti-affinity profile fleet-wide", "list the workload profiles". Dry-run by default; requires --commit --change-desc. Not for a single-workload fix (use workload-harden), read-only checks (use workload-audit / workload-findings), enforcement Triggers (use workload-guardrails), or propagating a fix to downstream variants (use promote-release).'
phase: act
allowed-tools: Bash(cub-workload --help) Bash(cub-workload * --help) Bash(cub auth status) Bash(cub-workload preflight) Bash(cub-workload readiness *) Bash(cub-workload findings *) Bash(cub-workload profile) Bash(cub-workload profile *) Bash(cub-workload fleet-edit *)
---

# workload-fleet

Apply a fix across **many** workloads and manage the reusable **profile library** — as data, dry-run by default. Two surfaces:

- **`profile install | list | apply`** — the profile library: named, parameterized edits stored as ConfigHub Invocations in the `common` Space (`resources-small/medium/large`, `harden-restricted`, `probes-http`, `anti-affinity-soft`, `termination-message-policy`). `install` seeds them (once per org); `apply <slug> <space>/<unit>` invokes one over a single workload.
- **`fleet-edit --profile <slug> [--where/shorthands] [--param]`** — applies a profile to *every* workload matching a selector in one server-side operation (the bulk analog of `profile apply`), scoped to workload kinds.

All **edit/create Units but do not publish them**.

## Why this matters

"Remediate the gaps across every prod workload" is a set-scale workflow no per-object validator does. `fleet-edit` runs one `InvokeStoredInvocation` over a `--where` selector (no client loop, comments preserved). A profile is one vocabulary for both a single `profile apply` and a bulk `fleet-edit`. Everything is **dry-run by default** and requires `--commit --change-desc`.

## When to use

- "List the available workload profiles." → `profile list`.
- "Set up the profile library." → `profile install`.
- "Apply the medium resource tier to workload X." → `profile apply resources-medium <space>/<unit> --param container=<name>`.
- "Harden every prod workload." → `fleet-edit --profile harden-restricted --environment prod`.
- "Set medium resources across the checkout component." → `fleet-edit --profile resources-medium --component checkout --param container=*`.

## Do not load for

- A single-workload fix — use **workload-harden**.
- Read-only checks — use **workload-audit** / **workload-findings** / **workload-availability**.
- Enforcement Triggers / guardrails — use **workload-guardrails**.
- Propagating a fix from a base Space to its variants — use **promote-release**.
- Publishing the Space's Release — that is `cub release publish <space>`.

## Preflight gates

1. `cub-workload preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. For `profile apply` / `fleet-edit`, the profile library exists — run `cub-workload profile install` once if `profile list` is empty.
3. The user has write permission on the target Spaces.

## The loop

1. **Scope + preview** (dry-run — the default):
   ```bash
   cub-workload profile list -o table
   cub-workload fleet-edit --profile harden-restricted --environment prod
   ```
   `fleet-edit` reports how many of the matched Units *would* change. Inspect the selector carefully — a broad `--where` touches many Units.
2. **Commit** with `--commit --change-desc`:
   ```bash
   cub-workload fleet-edit --profile harden-restricted --environment prod --commit \
     --change-desc "Harden prod workloads. User prompt: ..."
   ```
3. **Verify**: `cub-workload readiness --environment prod --failing-only` shows the dimension cleared.
4. **Roll out** is a separate step — hand off to `release-publish`.

## Stop conditions

- The selector is broader than intended (dry-run count surprises you) — narrow `--where` / shorthands before committing.
- An ApplyGate attaches on a Unit. **Do not bypass** — fix via **triggers-and-applygates**.
- A single workload is the real target — hand off to **workload-harden**.
- Variant propagation requested — hand off to **promote-release**.
- The user wants the change deployed — hand off to `release-publish`.

## Tool boundary

Allowed: `profile install|list|apply`, `fleet-edit` (dry-run by default; `--commit` passes `--change-desc`), and read commands. Not allowed: bypassing gates, `kubectl` mutations, publishing a Release.

## References

- `cub-workload profile --help`, `cub-workload fleet-edit --help`.
- Companion skills: **workload-harden**, **workload-audit**, `promote-release`, `triggers-and-applygates`, `release-publish`, `rollback-revision`.
