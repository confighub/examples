---
name: namespace-promote
description: 'Promote a namespace-envelope change from a base Space to every variant Space it was cloned into, with the cub-namespace CLI — an override-preserving upstream upgrade. Use for "roll out the tightened default-deny to all environments", "promote the envelope change to the downstream spaces", "upgrade the namespace policy Units to upstream head", "push the new required label everywhere". Dry-run by default; requires --commit --change-desc. Not for one-off edits (use namespace-backfill) or applying to a cluster (use cub-apply).'
phase: act
allowed-tools: Bash(cub-namespace --help) Bash(cub-namespace * --help) Bash(cub auth status) Bash(cub-namespace preflight) Bash(cub-namespace promote *)
---

# namespace-promote

Roll an envelope change forward: an override-preserving upgrade of the Units that are **behind their upstream** (the variant-propagation path). A default-deny or baseline-RBAC change authored in a base Space flows to every component and cluster Space cloned from it, keeping each Space's local customizations.

## Why this matters

When you tighten the envelope in one place — a stricter default-deny, a new required label, an RBAC change — every variant Space that was cloned from it is now behind. `promote` upgrades those downstream Units to the upstream head via a merge that preserves each Space's overrides. It edits Units; **rolling out to a cluster is a separate `cub unit apply`**.

## When to use

- "Roll out the tightened default-deny to all environments."
- "Promote the envelope change to the downstream spaces."
- "Upgrade the namespace policy Units to upstream head."

## Do not load for

- A one-off edit to a single Space's envelope — use **namespace-backfill**.
- Applying Units to a cluster — that is `cub-apply`.
- Read-only "what's behind upstream?" with no intent to promote — you can still dry-run `promote`, but a pure audit is **namespace-audit**.

## Preflight gates

1. `cub-namespace preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. The user has write permission on the downstream Space(s).

## The loop

1. **Preview** (dry-run is the default) — scope to the Units you mean with `--where` or a label shorthand:
   ```bash
   cub-namespace promote --component apptique
   cub-namespace promote --where "Slug LIKE 'default-deny-%'"
   ```
   It reports which Units would upgrade (those with `UpstreamRevisionNum` behind the upstream head), or that nothing is behind upstream.
2. **Commit** with `--commit --change-desc`:
   ```bash
   cub-namespace promote --component apptique --commit \
     --change-desc "Promote tightened default-deny to apptique variants. User prompt: ..."
   ```
3. **Roll out** is a separate, deliberate step — hand off to `cub-apply`.

## Stop conditions

- The upgrade would leave merge conflicts — resolve the Unit data first (via **namespace-backfill** / `cub-mutate`), then re-promote.
- An ApplyGate attaches after promotion — **do not bypass**; fix the data or the Trigger (via **triggers-and-applygates**).
- The user wants to apply to a cluster — hand off to `cub-apply`.

## Tool boundary

Allowed: `promote` (dry-run by default; `--commit` passes `--change-desc`). Not allowed: bypassing gates, `kubectl` mutations, applying to clusters.

## References

- `cub-namespace promote --help`.
- Companion skills: **namespace-backfill**, **promote-release**, `cub-apply`, `rollback-revision`.
