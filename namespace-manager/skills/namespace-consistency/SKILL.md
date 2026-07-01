---
name: namespace-consistency
description: 'Check whether a component''s namespace name and pod-security level are identical across all of its variant Spaces (environments / regions / clusters) in ConfigHub, using the cub-namespace CLI. Use for "is our namespace the same across dev/staging/prod?", "which components have drifted namespace names?", "is pod-security consistent across variants of X?", "check cross-variant namespace consistency". Not for per-namespace envelope completeness (use namespace-audit) or the full ranked findings list (use namespace-findings); read-only, ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-namespace --help) Bash(cub-namespace * --help) Bash(cub auth status) Bash(cub-namespace preflight) Bash(cub-namespace consistency) Bash(cub-namespace consistency *)
---

# namespace-consistency

Report, per component, whether its **namespace name** and **pod-security enforce level** are identical across every variant Space it lives in. It never mutates.

## Why this matters

The invariant is that a component uses the *same* namespace everywhere — dev, staging, every prod region and cluster. Whether that actually holds is a property of a **set of Spaces** (the component's variants), which no per-cluster controller and no per-resource validator can see: each sees only one cluster or one object. `cub-namespace consistency` groups the fleet's namespaces by their Space's `Component` label and compares across the variants. This is the read side of the namespace-name invariant; enforcing it is a separate per-cluster `set-namespace` Trigger. Only Spaces carrying a `Component` label participate; canonical base Spaces are excluded.

## When to use

- "Is the apptique namespace the same in dev and prod?"
- "Which components have drifted namespace names across environments/regions?" → `consistency --inconsistent-only`.
- "Is pod-security consistent across variants of component X?" → `consistency --component-name X`.
- "Check cross-variant namespace consistency across the fleet."

## Do not load for

- "Is namespace X missing a default-deny / pod-security / baseline RBAC?" — per-namespace completeness (use **namespace-audit**).
- "Give me the ranked list of everything wrong" — governance findings (use **namespace-findings**).
- Live cluster state — `kubectl`.

## Preflight gates

1. `cub-namespace preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

### Cross-variant consistency — `cub-namespace consistency`

Per component: the variant Spaces, the distinct namespace name(s) and pod-security level(s) seen across them, and whether they are consistent (plus a description of any drift).

```bash
cub-namespace consistency -o table
cub-namespace consistency --inconsistent-only -o table   # just the drift
cub-namespace consistency --component-name apptique
```

A component is **inconsistent** when its variants disagree on the namespace name (the invariant is broken — the fix is `set-namespace` per variant, a later write skill) or on the pod-security level.

### Scoping the fleet

Scope server-side with a single Unit `--where` predicate (Unit / `Space.*` / `Target.*`), plus the label shorthands `--component`, `--environment`, `--region`, `--owner`, `--layer`, `--variant` (each compiles to `Space.Labels.<Key> = '<value>'`), AND-joined. ConfigHub `where` is flat AND-only — no parentheses, no `OR` (a parenthesized clause fails with `invalid attribute name`). Example: `--where "Target.ProviderType = 'OCI'"` (the ProviderType recommended for ArgoCD/Flux).

Note: `--component` (server-side, `Space.Labels.Component`) scopes which Units are fetched; `--component-name` is a client-side display filter over the computed components. Consistency still needs to see *all* of a component's variant Spaces, so scoping narrower than a whole component can hide drift — prefer scoping by environment/region, not by a single Space.

## Stop conditions

- No components carry a `Component` Space label — report it; consistency needs the label to group variants (see the standard Space-label convention).
- The user wants to *fix* a drifted namespace — hand off to the write skills (`set-namespace` per variant).

## Tool boundary

Read-only. Fixing drift lives in the write skills. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-namespace consistency --help`.
- Companion skills: **namespace-audit**, **namespace-findings**.
