# cub-workload — Workload posture manager for ConfigHub

`cub-workload` manages the cross-cutting **security and reliability posture of
Kubernetes workloads** — container/pod security context, resource requests and
limits, probes, and availability (PodDisruptionBudget coverage plus pod
anti-affinity / topology spread) — stored as data in ConfigHub across a whole
fleet of cluster-Spaces. It is designed for use by an AI agent in a terminal, and
is a sibling of [`rbac-manager-for-agents`](../rbac-manager-for-agents),
[`network-policy-manager`](../network-policy-manager), and
[`namespace-manager`](../namespace-manager).

**Why a manager, not just a validator?** Per-object validators (kube-score,
kube-linter) already flag "runs as root" or "no limits" on a single object. What
they don't do is fix it, or reason over the fleet. `cub-workload` reads the whole
ConfigHub-managed set into a per-workload **production-readiness scorecard**, and
fixes gaps **as data** through reusable, parameterized **profiles** — committing
new Unit revisions and remediating across a selector, so clusters converge
through the normal apply pipeline with no drift. Its edge over the validators is
the *fix and the fleet*, not the finding. (One structural point: wired as a
per-Unit validating Trigger under one-resource-per-Unit, a validator sees a lone
workload Unit and false-positives on cross-Unit checks like "has a PDB" — the
manager does that set-join itself.)

## Commands

All read commands default to JSON (`-o json`); pass `-o table` for a human view.
Write commands are **dry-run by default** and require `--commit --change-desc`;
they edit/create Units but never apply to a cluster (that's a separate `cub unit
apply`), and never bypass ApplyGates.

| Command | Kind | What it does |
|---|---|---|
| `preflight` / `version` | diag | Verify the ConfigHub session; print version |
| `snapshot` | read | Per-cluster inventory of workloads + PodDisruptionBudgets |
| `list` | read | Enumerate workloads and PDBs across the fleet |
| `readiness` | read | Per-workload scorecard: security / resources / probes / hygiene |
| `availability` | read | Multi-replica workloads lacking a matching PDB and/or anti-affinity/spread |
| `findings` | read | Severity-ranked readiness findings across all analyzers |
| `harden` | write | Apply security-context + automount defaults to a workload |
| `set-resources` | write | Set container requests/limits (by tier or explicit) |
| `set-probes` | write | Add liveness/readiness/startup probe defaults |
| `ensure-pdb` | write | Author a PDB whose selector is derived from the workload |
| `ensure-spread` | write | Add pod anti-affinity or topology spread |
| `profile install\|list\|apply` | write | The profile library (parameterized Invocations) |
| `fleet-edit` | write | Bulk remediation: apply a profile across a `--where` selector |
| `promote` | write | Override-preserving upgrade of downstream Units to upstream |
| `guardrails install\|status\|annotate` | write | Enforcement pack — `vet-cel` per-resource + annotate-then-validate for PDB coverage |

## Agent skills

The `skills/` directory holds ConfigHub agent skills (a `SKILL.md` plus `evals/`)
that teach an agent when and how to drive the CLI:

| Skill | Kind | Covers |
|---|---|---|
| `workload-audit` | read | `snapshot`, `list`, `readiness` — inventory + the readiness scorecard |
| `workload-availability` | read | `availability` — PDB coverage + anti-affinity/spread |
| `workload-findings` | read | `findings` — severity-ranked triage |
| `workload-harden` | write | per-workload fixes: `harden`, `set-resources`, `set-probes`, `ensure-pdb`, `ensure-spread` |
| `workload-fleet` | write | `profile`, `fleet-edit`, `promote` — the library + bulk remediation + promotion |
| `workload-guardrails` | write | `guardrails install\|status\|annotate` — enforcement |

## Tutorial

[TUTORIAL.md](TUTORIAL.md) walks the whole flow end to end with real commands and
output — find unready workloads, fix them as data, remediate a whole environment,
and stand up enforcement.

## Build

```
make build      # -> bin/cub-workload
```

It runs standalone (`bin/cub-workload ...`) or as a cub plugin (`cub workload
...`). ConfigHub I/O uses your existing cub session — run `cub auth login` first.
