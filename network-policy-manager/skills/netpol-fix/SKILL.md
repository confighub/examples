---
name: netpol-fix
description: 'Fix NetworkPolicy coverage gaps as data in ConfigHub with the cub-netpol CLI: create a default-deny for a namespace, author an allow policy between two workloads, derive allow policies from the ConfigHub dependency graph (Links), or patch an existing policy (add DNS egress, except the cloud-metadata IP). Use for "lock down the payments namespace", "allow frontend to reach checkout", "generate allow policies from our Links", "add the DNS allowance to this egress policy". All writes are dry-run by default and require --commit with a --change-desc; Units are created/edited but NOT applied to a cluster. Not for inventory/findings (use netpol-audit/netpol-findings), fleet-wide remediation (use netpol-fleet), or installing enforcement (use netpol-guardrails).'
phase: act
allowed-tools: Bash(cub-netpol --help) Bash(cub-netpol * --help) Bash(cub auth status) Bash(cub-netpol preflight) Bash(cub-netpol coverage *) Bash(cub-netpol findings *) Bash(cub-netpol who-can-reach *) Bash(cub unit get *) Bash(cub-netpol default-deny *) Bash(cub-netpol allow *) Bash(cub-netpol allow-from-links *) Bash(cub-netpol fix *)
---

# netpol-fix

The config-as-data fix path for NetworkPolicy: close a coverage gap or restore intended connectivity by **editing the source of record**, so the cluster converges through the normal apply pipeline with no drift. Every write is **dry-run by default**; nothing is written until you re-run with `--commit` and a `--change-desc`, and even then the Unit is created/edited but **not applied** to a cluster.

## Why this matters

A per-resource validator can only *report* a gap; fixing it means editing the cluster out-of-band, which drifts. Here the fix is a new or edited ConfigHub Unit — versioned, reviewable, and rolled out deliberately later.

## When to use

- "Lock down the payments namespace" / "add a default-deny" → `default-deny <namespace>`.
- "Allow frontend to reach checkout" → `allow <src> <dst>`.
- "Generate allow policies from our dependency graph / Links" → `allow-from-links`.
- "This egress default-deny breaks DNS — add the allowance" → `fix dns <space>/<unit>`.
- "Except the cloud-metadata IP from this wide egress" → `fix metadata <space>/<unit>`.

## Do not load for

- Inventory / coverage / findings — **netpol-audit**, **netpol-findings**.
- Fixing the *whole fleet* at once — **netpol-fleet** (`fleet default-deny`, `promote`).
- Installing enforcement (Triggers/ApplyWarnings) — **netpol-guardrails**.
- Applying the change to a cluster — that's `cub unit apply` (the **cub-apply** skill); this skill only edits stored config.

## Preflight gates

1. `cub-netpol preflight` succeeds. If not, ask the user to run `cub auth login` and retry.
2. The user has Edit/Create permission on the target Space (the commit fails server-side otherwise — report the error, don't retry blindly).

## The loop

1. **Find the gap** with **netpol-audit** / **netpol-findings** (`coverage`, `findings`).
2. **Preview (dry-run)** — run the write with no `--commit`; cub-netpol prints the generated YAML or the planned edit and writes nothing:
   ```bash
   cub-netpol default-deny payments --cluster prod-cluster                 # ingress default-deny
   cub-netpol default-deny payments --cluster prod-cluster --egress         # + egress, allowing DNS
   cub-netpol allow frontend checkout --cluster prod-cluster --port 8080
   cub-netpol allow-from-links --space apptique-prod                        # one policy per dest, from Links
   cub-netpol fix dns apptique-prod/default-deny-apptique
   cub-netpol fix metadata apptique-prod/wide-egress
   ```
3. **Confirm** the YAML/edit matches intent. Adjust flags if not.
4. **Commit** with a real change description:
   ```bash
   cub-netpol default-deny payments --cluster prod-cluster \
     --commit --change-desc "Default-deny ingress for payments (no prior policy). User prompt: ..."
   ```
5. **Stop.** The write created/edited a Unit; it is NOT applied. To roll it out, hand off to **cub-apply** (`cub unit apply`), which respects ApplyGates.

## Notes & idioms

- **default-deny** infers the Space/cluster Target from the namespace's workloads; use `--cluster`/`--space` to disambiguate. `--egress` includes the kube-dns `:53` allowance so it doesn't break name resolution.
- **allow** is ingress on the destination by default (`--egress` for source-side egress); `--port` restricts it.
- **allow-from-links** reads ConfigHub Links (consumer→producer) and authors one consolidated policy per destination (all sources as `from` peers) — the idiomatic shape; `--per-edge` for one policy per pair. Re-running **upserts** (adds only missing sources to an existing policy; idempotent). A Link to a multi-resource Unit targets the resource whose name matches the Unit slug.
- **fix dns / metadata** are surgical, idempotent `set-yq` edits; each says "nothing to do" when already satisfied.

## Stop conditions

- The user asks to apply/roll out — hand off to **cub-apply**, don't apply here.
- A commit hits an ApplyGate or permission error — report it; fix the data or route to **triggers-and-applygates**; never bypass a gate.
- Whole-fleet remediation requested — hand off to **netpol-fleet**.

## Tool boundary

Allowed: the dry-run/commit writes above (always with `--change-desc` on commit) plus read commands for context. Not allowed: applying to a cluster, `kubectl` mutation, bypassing gates.

## References

- `cub-netpol default-deny --help`, `cub-netpol allow --help`, `cub-netpol allow-from-links --help`, `cub-netpol fix --help`.
- Companion skills: **netpol-audit**, **netpol-findings**, **netpol-fleet**, **netpol-guardrails**, **cub-apply**.
