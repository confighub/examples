---
name: workload-fleet
description: 'Fleet-scale workload remediation and the reusable profile library with the cub-workload CLI — profile install/list/apply (parameterized Invocations: resource tiers, harden, probes, anti-affinity, termination policy), fleet-edit (apply a profile across a --where selector of workloads in one operation), and promote (override-preserving upgrade of downstream Units to their upstream head). Use for "harden every prod workload", "set medium resources across the checkout component", "apply the anti-affinity profile fleet-wide", "roll the workload fix downstream", "list the workload profiles". Dry-run by default; requires --commit --change-desc. Not for a single-workload fix (use workload-harden), read-only checks (use workload-audit / workload-findings), or enforcement Triggers (use workload-guardrails).'
phase: act
allowed-tools: Bash(cub-workload --help) Bash(cub-workload * --help) Bash(cub auth status) Bash(cub-workload preflight) Bash(cub-workload readiness *) Bash(cub-workload findings *) Bash(cub-workload profile) Bash(cub-workload profile *) Bash(cub-workload fleet-edit *) Bash(cub-workload promote *)
---

# workload-fleet

Apply a fix across **many** workloads and manage the reusable **profile library** — as data, dry-run by default. Three surfaces:

- **`profile install | list | apply`** — the profile library: named, parameterized edits stored as ConfigHub Invocations in the `workload-profiles` Space (`resources-small/medium/large`, `harden-restricted`, `probes-http`, `anti-affinity-soft`, `termination-message-policy`). `install` seeds them (once per org); `apply <slug> <space>/<unit>` invokes one over a single workload.
- **`fleet-edit --profile <slug> [--where/shorthands] [--param]`** — applies a profile to *every* workload matching a selector in one server-side operation (the bulk analog of `profile apply`), scoped to workload kinds.
- **`promote`** — override-preserving upgrade of downstream Units that are behind their upstream (`UpstreamRevisionNum > 0`) — the variant-propagation path.

All **edit/create Units but do not apply them** to a cluster.

## Why this matters

"Remediate the gaps across every prod workload, then promote the fix" is a set-scale workflow no per-object validator does. `fleet-edit` runs one `InvokeStoredInvocation` over a `--where` selector (no client loop, comments preserved); `promote` carries a fix from a base Space to its downstream variants while preserving local customizations. A profile is one vocabulary for both a single `profile apply` and a bulk `fleet-edit`. Everything is **dry-run by default** and requires `--commit --change-desc`.

## When to use

- "List the available workload profiles." → `profile list`.
- "Set up the profile library." → `profile install`.
- "Apply the medium resource tier to workload X." → `profile apply resources-medium <space>/<unit> --param container=<name>`.
- "Harden every prod workload." → `fleet-edit --profile harden-restricted --environment prod`.
- "Set medium resources across the checkout component." → `fleet-edit --profile resources-medium --component checkout --param container=*`.
- "Roll the workload fix downstream to staging/prod." → `promote --environment staging` (or `--component X`).

## Do not load for

- A single-workload fix — use **workload-harden**.
- Read-only checks — use **workload-audit** / **workload-findings** / **workload-availability**.
- Enforcement Triggers / guardrails — use **workload-guardrails**.
- Applying Units to a cluster — that is `cub unit apply`.

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
4. **Promote** a fix from a base to downstream variants (dry-run then commit):
   ```bash
   cub-workload promote --component checkout --commit --change-desc "..."
   ```
5. **Roll out** is a separate step — hand off to `cub-apply`.

## Stop conditions

- The selector is broader than intended (dry-run count surprises you) — narrow `--where` / shorthands before committing.
- An ApplyGate attaches on a Unit. **Do not bypass** — fix via **triggers-and-applygates**.
- A single workload is the real target — hand off to **workload-harden**.
- The user wants to apply to a cluster — hand off to `cub-apply`.

## Tool boundary

Allowed: `profile install|list|apply`, `fleet-edit`, `promote` (dry-run by default; `--commit` passes `--change-desc`), and read commands. Not allowed: bypassing gates, `kubectl` mutations, applying to clusters.

## References

- `cub-workload profile --help`, `cub-workload fleet-edit --help`, `cub-workload promote --help`.
- Companion skills: **workload-harden**, **workload-audit**, `promote-release`, `triggers-and-applygates`, `cub-apply`, `rollback-revision`.
