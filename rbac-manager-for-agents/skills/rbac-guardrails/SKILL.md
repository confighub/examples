---
name: rbac-guardrails
description: 'Install and inspect the RBAC guardrail policy pack across a ConfigHub fleet with the cub-rbac CLI. Use for "enforce RBAC policy", "block wildcard roles / privilege escalation / cluster-admin bindings", "set up RBAC guardrails", "wire the guardrail triggers to my spaces", "which units have RBAC warnings?", "are our RBAC policies installed?". Installs three Warn=true Triggers in a policy Space, labeled Guardrails: rbac, and adds the rbac group to each in-scope Space''s WhereTrigger; dry-run by default, --commit to apply. Not for one-off validation (use rbac-findings), not for editing RBAC config (use rbac-edit); promoting a guardrail to blocking and applying changes are done with cub directly.'
phase: act
allowed-tools: Bash(cub-rbac --help) Bash(cub-rbac * --help) Bash(cub auth status) Bash(cub-rbac preflight) Bash(cub-rbac findings *) Bash(cub-rbac guardrails *)
---

# rbac-guardrails

Install and inspect a small pack of RBAC validation policies, defined once in a policy Space and enforced in each Space that opts into the `rbac` group. Installation is **dry-run by default**; nothing changes until you re-run with `--commit`.

## The pack

| trigger | blocks |
|---|---|
| `no-rbac-wildcards` | wildcard verbs / resources / apiGroups in a Role/ClusterRole |
| `no-rbac-privilege-escalation` | `escalate` / `bind` / `impersonate` verbs |
| `no-cluster-admin-binding` | ClusterRoleBindings to `cluster-admin` |

Triggers are created with **Warn=true** → they produce advisory **ValidationWarnings**, never blocking **ValidationErrors**. Installing on an existing fleet never blocks anyone. Promote one to blocking later (a single change, fleet-wide):

```bash
cub trigger update no-rbac-wildcards --space <policy-space> --unwarn
```

## Why this matters

This is the *enforcement* complement to **rbac-findings** (which only reports). The policies are defined once in a policy Space and labeled `Guardrails: rbac`. Each wired Space's `WhereTrigger` lists the groups it has opted into, from that one policy Space:

```
Space.Slug = 'common' AND Labels.Guardrails IN ('namespace', 'rbac') AND Labels.AppliedTo != 'Target'
```

So the rules run on every Mutation in those Spaces, another tool's group is added to a Space only when that tool's install wires it, and a Trigger labeled into the group in any other Space is never selected.

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
2. Installing requires permission to create Triggers in the policy Space and to update in-scope Spaces.

## The loop

1. **Plan (dry-run).** Always start here; it changes nothing:
   ```bash
   cub-rbac guardrails install -o table
   cub-rbac guardrails install --where-space "Labels.Environment = 'prod'" -o table   # narrow scope
   ```
   The plan lists the policy Space and group, which Spaces would be wired, which are already wired, and which are skipped (with the reason — a custom WhereTrigger, a TriggerFilterID, or their own Triggers). Spaces that select Triggers another way are **reported, not modified**.
2. **Review** the wire list and skips with the user. Skipped Spaces need the `rbac` group added to whatever they already select — call that out.
3. **Apply** once the plan looks right:
   ```bash
   cub-rbac guardrails install --commit
   cub-rbac guardrails install --where-space "Labels.Environment = 'prod'" --commit
   ```
   Install is idempotent; re-running updates the Triggers to this version of the rules and re-lists the Triggers of Spaces already wired.
4. **Verify** what the guardrails now flag:
   ```bash
   cub-rbac guardrails status -o table     # Units with ValidationWarnings / ValidationErrors
   ```
   To resolve a warning, hand off to **rbac-edit** (fix the config) or **rbac-findings** (understand it).

## Exceptions

A resource that is meant to break a rule — an operator that needs a wildcard ClusterRole, say — is excepted from that one rule with an annotation naming the rule, whose value says why:

```yaml
metadata:
  annotations:
    exceptions.guardrails.confighub.com/no-rbac-wildcards: "argocd manages every resource type"
```

Each rule's Trigger skips the resources carrying its exception annotation; the other rules still check them. Adding one is a config change for the user to make deliberately (`cub function set -- set-annotation` on the Unit, with `--where-resource` naming the one resource, as in `ConfigHub.ResourceName = '<namespace>/<name>'`, or `'/<name>'` for a cluster-scoped one), not something to add to silence a warning.

## Flags

- `--policy-space` (default `common`) — where the Triggers live.
- `--where-space` — ConfigHub filter to narrow which Spaces get wired.
- `--commit` — apply the plan (omit for dry-run). `-o table` / `-o json` for the plan/result.

## Tool boundary

- Allowed: `cub-rbac guardrails install` (dry-run + commit), `cub-rbac guardrails status`, `cub-rbac findings`.
- Not allowed: promoting to blocking, applying Units, deleting policy objects, raw `kubectl`. Those are deliberate `cub` actions for the user.

## Stop conditions

- Dry-run shows nothing to wire and everything already wired — report that the pack is installed; offer `guardrails status`.
- Spaces are skipped for custom trigger wiring — report them so the user can fold the `rbac` group into their existing selection.
- The user wants a guardrail to actually block (not warn) — point to `cub trigger update <slug> --space <policy-space> --unwarn`.

## References

- `cub-rbac guardrails install --help`, `cub-rbac guardrails status --help`.
- Companion skills: **rbac-findings** (advisory analysis), **rbac-edit** (fix violations), **triggers-and-applygates** (general Trigger/ValidationError mechanics).
