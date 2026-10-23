---
name: namespace-pod-security
description: 'Report which Pod Security Standard (privileged / baseline / restricted) each namespace''s workloads meet in ConfigHub, and which Units and resources keep a namespace from a stricter level, using the cub-namespace CLI. Use for "what pod security level can we enforce?", "which namespaces could be restricted?", "what is blocking baseline in monitoring?", "would enforcing restricted break anything?", "is any namespace''s enforce label rejecting its own pods?". Not for whether a Namespace has a pod-security label at all (use namespace-audit) or the ranked findings list (use namespace-findings); read-only, ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-namespace --help) Bash(cub-namespace * --help) Bash(cub auth status) Bash(cub-namespace preflight) Bash(cub-namespace pod-security) Bash(cub-namespace pod-security *)
---

# namespace-pod-security

Report, per namespace, the most restrictive Pod Security Standard level all its workloads meet, how that compares with the Namespace's `pod-security.kubernetes.io/enforce` label, and the workloads that block the next level. It never mutates.

## Why this matters

A namespace's enforce label decides which pods admission accepts. Raising it is only safe when every workload in the namespace already meets the new level, and that is a property of the whole set of the namespace's workloads, spread across Units and Spaces. `cub-namespace pod-security` runs the server's `vet-pod-security-standard` function (the same checks the PodSecurity admission controller runs) at `restricted` and `baseline` over the Units holding workloads, and joins the results per namespace.

## When to use

- "What pod security level could each namespace enforce?" → `pod-security -o table`.
- "Which namespaces could be tightened?" → `pod-security --status tightenable`.
- "Is any namespace enforcing a level its workloads fail?" → `pod-security --status violating`.
- "What keeps monitoring from baseline?" → `pod-security --namespace monitoring`.
- Before stamping pod-security defaults with `apply-envelope` (which sets `enforce: baseline`): check that the namespace's workloads meet baseline.

## Do not load for

- "Which namespaces have no pod-security label?" — envelope completeness (use **namespace-audit**).
- "Give me the ranked list of everything wrong" — governance findings (use **namespace-findings**).
- Live cluster state or admission warnings — `kubectl`.

## Preflight gates

1. `cub-namespace preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. The server has `vet-pod-security-standard`. If the command says the server has no such function, report that the server is too old for this check; do not fall back to reasoning about securityContexts by hand.

## The toolkit

### Pod-security posture — `cub-namespace pod-security`

Per namespace: workloads, the enforce label, the achievable level, a status, and the blockers for the next level (Space, Unit, Kind/name, and the failed checks).

```bash
cub-namespace pod-security -o table
cub-namespace pod-security --status violating -o table
cub-namespace pod-security --namespace monitoring
cub-namespace pod-security --version v1.30      # a Kubernetes minor version's standards instead of latest
```

Statuses:

| Status | Meaning |
|---|---|
| `violating` | the label enforces a level some workload fails: admission rejects that workload's pods |
| `unlabeled` | no enforce label, or no Namespace Unit to carry one |
| `tightenable` | the workloads meet a stricter level than the label enforces |
| `at-level` | the label enforces exactly the level the workloads meet |

`blockers` lists only what stands between the achievable level and the next one: a privileged namespace's blockers are its baseline failures. `checks[].message` is worded as admission words its rejection and names the containers and fields to change.

Units the function could not check are listed under `unchecked` with the reason; a namespace counting `unchecked` workloads has an achievable level that is only an upper bound.

### Scoping the fleet

Same server-side scoping as the other commands: `--where` (Unit / `Space.*` / `Target.*`, flat AND only) and the label shorthands `--component`, `--environment`, `--region`, `--owner`, `--layer`, `--variant`. `--cluster`, `--namespace` and `--status` filter the computed report client-side. Canonical base Spaces are excluded, as in every analysis.

## Stop conditions

- The user wants to *raise* a label — report the achievable level and blockers, and hand off to the write skills (`apply-envelope`, or a `set-label` edit on the Namespace Unit); do not edit here.
- The user wants a blocker *fixed* — name the Unit, resource and failed checks; changing a workload's securityContext belongs to the workload's owner.

## Tool boundary

Read-only: every function invocation is a dry run of a validating function. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-namespace pod-security --help`.
- Pod Security Standards: https://kubernetes.io/docs/concepts/security/pod-security-standards/
- Companion skills: **namespace-audit**, **namespace-findings**, **namespace-enforce**.
