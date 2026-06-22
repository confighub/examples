---
name: rbac-guardrails
description: 'Install and inspect the RBAC guardrail policy pack across a ConfigHub fleet with the cub-rbac CLI. Use for "enforce RBAC policy", "block wildcard roles / privilege escalation / cluster-admin bindings", "set up RBAC guardrails", "wire the guardrail triggers to my spaces", "which units have RBAC warnings?", "are our RBAC policies installed?". Installs three Warn=true Triggers in a policy Space and wires each in-scope Space''s TriggerFilterID to a shared Filter; dry-run by default, --commit to apply. Not for one-off validation (use rbac-findings), not for editing RBAC config (use rbac-edit); promoting a guardrail to blocking and applying changes are done with cub directly.'
phase: act
allowed-tools: Bash(cub-rbac --help) Bash(cub-rbac * --help) Bash(cub auth status) Bash(cub-rbac preflight) Bash(cub-rbac findings *) Bash(cub-rbac guardrails *)
---

# rbac-guardrails

Install and inspect a small pack of RBAC validation policies, defined once in a policy Space and enforced fleet-wide via a shared Trigger Filter. Installation is **dry-run by default**; nothing changes until you re-run with `--commit`.

## The pack

| trigger | blocks |
|---|---|
| `no-rbac-wildcards` | wildcard verbs / resources / apiGroups in a Role/ClusterRole |
| `no-rbac-privilege-escalation` | `escalate` / `bind` / `impersonate` verbs |
| `no-cluster-admin-binding` | ClusterRoleBindings to `cluster-admin` |

Triggers are created with **Warn=true** → they produce advisory **ApplyWarnings**, never blocking **ApplyGates**. Installing on an existing fleet never blocks anyone. Promote one to blocking later (a single change, fleet-wide):

```bash
cub trigger update no-rbac-wildcards --space <policy-space> --unwarn
```

## Why this matters

This is the *enforcement* complement to **rbac-findings** (which only reports). Defining the policies once in a policy Space and pointing every Space's `TriggerFilterID` at one shared Filter means the rules run on every Mutation across the fleet, and adding a policy later updates everywhere at once.

## When to use

- "Set up / install RBAC guardrails" / "enforce RBAC policy across the fleet."
- "Block wildcard roles / privilege escalation / cluster-admin bindings."
- "Are our RBAC policies installed / which Spaces are wired?"
- "Which Units have RBAC warnings or gates right now?" → `guardrails status`.

## Do not load for

- One-off, non-installed validation of current config — use **rbac-findings**.
- Editing RBAC config to fix a violation — use **rbac-edit**.
- Promoting a guardrail to blocking (`cub trigger update --unwarn`) or applying Units — those are `cub` operations, done deliberately by the user.

## Preflight gates

1. `cub-rbac preflight` succeeds (cub installed, ConfigHub session valid). If not, ask the user to run `cub auth login` and retry.
2. Installing requires permission to create Triggers/Filters in the policy Space and to update in-scope Spaces.

## The loop

1. **Plan (dry-run).** Always start here; it changes nothing:
   ```bash
   cub-rbac guardrails install -o table
   cub-rbac guardrails install --where-space "Labels.Environment = 'prod'" -o table   # narrow scope
   ```
   The plan lists the policy Space and Filter, which Spaces would be wired, which are already wired, and which are skipped (with the reason — a custom WhereTrigger, a different TriggerFilterID, or their own Triggers). Spaces that select Triggers another way are **reported, not modified**.
2. **Review** the wire list and skips with the user. Skipped Spaces need the guardrail Filter added to whatever they already select — call that out.
3. **Apply** once the plan looks right:
   ```bash
   cub-rbac guardrails install --commit
   cub-rbac guardrails install --where-space "Labels.Environment = 'prod'" --commit
   ```
   Install is idempotent (`--allow-exists`); re-running is safe.
4. **Verify** what the guardrails now flag:
   ```bash
   cub-rbac guardrails status -o table     # Units with ApplyWarnings / ApplyGates
   ```
   To resolve a warning, hand off to **rbac-edit** (fix the config) or **rbac-findings** (understand it).

## Flags

- `--policy-space` (default `policy-guardrails`) — where the Triggers + Filter live.
- `--where-space` — ConfigHub filter to narrow which Spaces get wired.
- `--commit` — apply the plan (omit for dry-run). `-o table` / `-o json` for the plan/result.

## Tool boundary

- Allowed: `cub-rbac guardrails install` (dry-run + commit), `cub-rbac guardrails status`, `cub-rbac findings`.
- Not allowed: promoting to blocking, applying Units, deleting policy objects, raw `kubectl`. Those are deliberate `cub` actions for the user.

## Stop conditions

- Dry-run shows nothing to wire and everything already wired — report that the pack is installed; offer `guardrails status`.
- Spaces are skipped for custom trigger wiring — report them so the user can fold the guardrail Filter into their existing selection.
- The user wants a guardrail to actually block (not warn) — point to `cub trigger update <slug> --space <policy-space> --unwarn`.

## References

- `cub-rbac guardrails install --help`, `cub-rbac guardrails status --help`.
- Companion skills: **rbac-findings** (advisory analysis), **rbac-edit** (fix violations), **triggers-and-applygates** (general Trigger/ApplyGate mechanics).
