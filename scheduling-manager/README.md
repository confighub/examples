# cub-scheduling — Workload placement manager for ConfigHub

`cub-scheduling` manages **where Kubernetes workloads are allowed to land** —
`nodeSelector`, `tolerations`, and node affinity — stored as data in ConfigHub
across a whole fleet of cluster-Spaces. It is designed for use by an AI agent in a
terminal, and is a sibling of
[`rbac-manager-for-agents`](../rbac-manager-for-agents),
[`network-policy-manager`](../network-policy-manager),
[`namespace-manager`](../namespace-manager), and
[`workload-manager`](../workload-manager).

**Placement, not availability.** This tool governs *which node a pod lands on*.
Spreading a workload's own replicas so a node/zone loss can't take them all — pod
anti-affinity and topology spread — is an *availability* concern owned by
[`workload-manager`](../workload-manager), not here.

**A toleration is not placement.** A toleration only *permits* scheduling onto a
tainted node; it doesn't pin the pod there. `cub-scheduling` treats a workload as
"constrained" only when it has a `nodeSelector` or a required node affinity, and
flags workloads that tolerate a taint but constrain nothing.

## Tutorial

[TUTORIAL.md](TUTORIAL.md) walks the whole flow end to end with real commands and
output — audit placement, find the anti-pattern, fix it as data, and enforce it.

## Build

```
make build      # -> bin/cub-scheduling
```

Runs standalone (`bin/cub-scheduling ...`) or as a cub plugin (`cub scheduling
...`). ConfigHub I/O uses your existing cub session — run `cub auth login` first.

## Commands

Read commands default to JSON (`-o json`); pass `-o table` for a human view. Write
commands are **dry-run by default** and require `--commit --change-desc`; they edit
Units but never apply to a cluster (that's a separate `cub unit apply`), and never
bypass ApplyGates.

| Command | Kind | What it does |
|---|---|---|
| `preflight` / `version` | diag | Verify the ConfigHub session; print version |
| `snapshot` | read | Per-cluster workload counts + how many pin placement |
| `list` | read | Enumerate pod-bearing workloads across the fleet |
| `placement` | read | Per-workload nodeSelector / tolerations / node affinity + constrained? |
| `findings` | read | Severity-ranked placement findings |
| `set-node-selector` | write | Set the pod-template nodeSelector |
| `set-tolerations` | write | Set the pod-template tolerations |
| `set-node-affinity` | write | Set a required node affinity term |
| `profile install\|list\|apply` | write | The placement profile library (parameterized Invocations) |
| `fleet-edit` | write | Apply a placement profile across a `--where` selector |
| `promote` | write | Override-preserving upgrade of downstream Units to upstream |
| `guardrails install\|status` | write | Enforcement pack — a `vet-cel` "toleration needs placement" rule |

## Agent skills

The `skills/` directory holds ConfigHub agent skills (a `SKILL.md` plus `evals/`):

| Skill | Kind | Covers |
|---|---|---|
| `scheduling-audit` | read | `snapshot`, `list`, `placement` |
| `scheduling-findings` | read | `findings` |
| `scheduling-place` | write | `set-node-selector`, `set-tolerations`, `set-node-affinity`, `profile`, `fleet-edit`, `promote` |
| `scheduling-guardrails` | write | `guardrails install\|status` |
