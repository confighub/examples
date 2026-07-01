---
name: namespace-findings
description: 'Run the ranked namespace-governance analyzer set over a ConfigHub fleet with the cub-namespace CLI — envelope gaps (missing default-deny / pod-security / baseline RBAC / Namespace object), duplicate namespaces colliding on a Target, and cross-variant name/pod-security inconsistency. Use for "what''s wrong with our namespaces?", "namespace governance findings", "any duplicate namespaces?", "show high-severity namespace issues", "audit namespace hygiene". Not for raw inventory (use namespace-audit) or a single component''s consistency detail (use namespace-consistency); read-only, ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-namespace --help) Bash(cub-namespace * --help) Bash(cub auth status) Bash(cub-namespace preflight) Bash(cub-namespace findings) Bash(cub-namespace findings *)
---

# namespace-findings

Run the v1 analyzer set over the fleet and return a **severity-ranked** list of namespace-governance findings. It never mutates.

## Why this matters

Each finding is a property of the *whole set* of resources — "this namespace has workloads but no default-deny", "two Units define the same namespace on one cluster", "this component's namespace name drifts across variants" — none of which a per-resource validator or a single-cluster controller can determine. `cub-namespace` computes them over the fleet's ConfigHub-managed Units and ranks by severity. Output is JSON by default; add `-o table`.

## The analyzers

| Analyzer | Severity | Fires when |
| --- | --- | --- |
| `missing-default-deny` | high | occupied namespace with no default-deny NetworkPolicy |
| `duplicate-namespace` | high | two Namespace Units collide on name + Target (one cluster) |
| `missing-namespace-object` | medium | occupied namespace with no `v1/Namespace` Unit |
| `missing-pod-security` | medium | Namespace object with no `pod-security.kubernetes.io/enforce` label |
| `namespace-name-inconsistent` | medium | a component's namespace name varies across its variants |
| `missing-baseline-rbac` | low | occupied namespace with no RoleBinding |
| `pod-security-inconsistent` | low | a component's pod-security level varies across its variants |

## When to use

- "What's wrong with our namespaces?" / "audit namespace hygiene / governance."
- "Any duplicate namespaces?" → `findings --analyzer duplicate-namespace`.
- "Show the high-severity namespace issues." → `findings --severity high`.
- "Namespace findings for prod-cluster." → `findings --cluster prod-cluster`.

## Do not load for

- Raw inventory or per-namespace envelope detail — use **namespace-audit**.
- One component's cross-variant consistency detail (the variant list) — use **namespace-consistency**.
- Live cluster state — `kubectl`.
- Fixing what's found — the write skills.

## Preflight gates

1. `cub-namespace preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

```bash
cub-namespace findings -o table
cub-namespace findings --severity high
cub-namespace findings --analyzer missing-default-deny
cub-namespace findings --cluster prod-cluster
```

`--severity` (high|medium|low), `--analyzer`, and `--cluster` filter the output client-side.

### Scoping the fleet

Scope server-side with a single Unit `--where` predicate (Unit / `Space.*` / `Target.*`), plus the label shorthands `--component`, `--environment`, `--region`, `--owner`, `--layer`, `--variant` (each compiles to `Space.Labels.<Key> = '<value>'`), AND-joined. ConfigHub `where` is flat AND-only — no parentheses, no `OR` (a parenthesized clause fails with `invalid attribute name`). Example: `--where "Target.ProviderType = 'OCI'"` (the ProviderType recommended for ArgoCD/Flux). `--cluster` is a client-side display filter.

## Stop conditions

- Snapshot empty — report it; suggest widening scope or checking `cub auth status` / org context.
- The user wants the variant-by-variant detail behind an inconsistency finding — hand off to **namespace-consistency**.
- The user wants to *fix* a finding — hand off to the write skills.

## Tool boundary

Read-only analysis. Fixing lives in the write skills. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-namespace findings --help`.
- Companion skills: **namespace-audit**, **namespace-consistency**.
