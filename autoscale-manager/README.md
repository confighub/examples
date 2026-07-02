# cub-autoscale — Autoscaling manager for ConfigHub

`cub-autoscale` manages **Kubernetes autoscaling** — HorizontalPodAutoscalers and
KEDA ScaledObjects — stored as data in ConfigHub across a whole fleet of
cluster-Spaces. It is designed for use by an AI agent in a terminal, and is a
sibling of
[`rbac-manager-for-agents`](../rbac-manager-for-agents),
[`network-policy-manager`](../network-policy-manager),
[`namespace-manager`](../namespace-manager),
[`workload-manager`](../workload-manager),
[`scheduling-manager`](../scheduling-manager), and
[`observability-manager`](../observability-manager).

**Fix-as-data, plus the checks a single-resource validator can't make.** Beyond
per-resource issues (a pinned autoscaler where `min == max`), `cub-autoscale`
joins across resources: it flags a PodDisruptionBudget whose `minAvailable`
would block all voluntary eviction at an autoscaler's `minReplicas`, so node
drains stall at minimum scale.

**HPA → KEDA, client-side.** `convert-keda` rewrites an HPA as an equivalent KEDA
ScaledObject by running the `convert-hpa-to-keda` function in an **embedded
executor in-process** — it fetches the Unit's data, runs the function locally, and
writes the result back. No server-side function and no worker are required.

## Tutorial

[TUTORIAL.md](TUTORIAL.md) walks the whole flow end to end with real commands and
output — audit autoscaling, find the anti-patterns, edit an HPA, convert it to
KEDA, and enforce it.

## Build

```
make build      # -> bin/cub-autoscale
```

Runs standalone (`bin/cub-autoscale ...`) or as a cub plugin (`cub autoscale
...`). ConfigHub I/O uses your existing cub session — run `cub auth login` first.

## Commands

Read commands default to JSON (`-o json`); pass `-o table` for a human view. Write
commands are **dry-run by default** and require `--commit --change-desc`; they edit
Units but never apply to a cluster (that's a separate `cub unit apply`), and never
bypass ApplyGates.

| Command | Kind | What it does |
|---|---|---|
| `preflight` / `version` | diag | Verify the ConfigHub session; print version |
| `snapshot` | read | Per-cluster HPA / ScaledObject / workload / PDB counts |
| `list` | read | Enumerate HPAs and ScaledObjects across the fleet |
| `findings` | read | Severity-ranked autoscaling findings (incl. PDB-vs-minReplicas) |
| `set-hpa` | write | Edit an HPA's min/max replicas and cpu/memory targets |
| `convert-keda` | write | Rewrite an HPA as a KEDA ScaledObject (embedded executor) |
| `profile install\|list\|apply` | write | The autoscaling profile library (parameterized Invocations) |
| `fleet-edit` | write | Apply an autoscaling profile across a `--where` selector |
| `promote` | write | Override-preserving upgrade of downstream Units to upstream |
| `guardrails install\|status` | write | Enforcement pack — `not-pinned` (vet-cel) + `schema-valid` (vet-schemas) |

## Agent skills

The `skills/` directory holds ConfigHub agent skills (a `SKILL.md` plus `evals/`):

| Skill | Kind | Covers |
|---|---|---|
| `autoscale-audit` | read | `snapshot`, `list` |
| `autoscale-findings` | read | `findings` |
| `autoscale-edit` | write | `set-hpa`, `convert-keda`, `profile`, `fleet-edit`, `promote` |
| `autoscale-guardrails` | write | `guardrails install\|status` |
