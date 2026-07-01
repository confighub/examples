---
name: workload-findings
description: 'Produce a severity-ranked list of Kubernetes workload-readiness findings across a ConfigHub fleet with the cub-workload CLI, spanning every analyzer (security, resources, probes, hygiene, availability). Use for "what is wrong with our workloads, ranked?", "show the high-severity workload issues", "workload findings for the payments component", "audit workload readiness across the fleet". Not for the raw per-workload scorecard (use workload-audit), PDB-only coverage (use workload-availability), or fixing issues (use workload-harden); analysis is over ConfigHub-managed Units only.'
phase: verify
allowed-tools: Bash(cub-workload --help) Bash(cub-workload * --help) Bash(cub auth status) Bash(cub-workload preflight) Bash(cub-workload findings) Bash(cub-workload findings *)
---

# workload-findings

Flatten the fleet readiness scorecard into a single **severity-ranked list** of findings — most-severe first — spanning every analyzer. This is the triage surface: "what should we fix, in what order." Read-only.

## Why this matters

`readiness` gives a per-workload scorecard and `availability` a PDB report; `findings` unifies both into one ranked stream so an operator (or agent) can work top-down. Severity: a failing **security / resources / availability** check is `high`; a failing **probe** is `medium`; warnings are one step down; **hygiene** is `low`. Analysis is over **ConfigHub-managed Units only**.

## When to use

- "What's wrong with our workloads, ranked?" → `findings`.
- "Show the high-severity issues." → `findings --severity high`.
- "Only the security findings." → `findings --analyzer security`.
- "Findings for the payments component / in prod." → `findings --component payments` / `--environment prod`.

## Do not load for

- The raw per-workload scorecard — use **workload-audit** (`readiness`).
- PDB coverage / spread specifically — use **workload-availability**.
- Fixing findings — use **workload-harden** (single workload) or **workload-fleet** (bulk).
- Live cluster state — `kubectl`.

## Preflight gates

1. `cub-workload preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.

## The toolkit

### Ranked findings — `cub-workload findings`

One finding per issue, sorted high → low, each carrying the analyzer, cluster, namespace, kind, name, and message.

```bash
cub-workload findings -o table
cub-workload findings --severity high
cub-workload findings --analyzer availability --environment prod
```

### Scoping

Server-side `--where` + label shorthands (`--component`, `--environment`, …); plus `--severity` (high|medium|low) and `--analyzer` (security|resources|probes|hygiene|availability); `--cluster` / `--namespace` are client-side.

## Stop conditions

- Zero findings in scope — report the fleet is clean for that scope.
- The user wants to fix something — hand off to **workload-harden** / **workload-fleet**.

## Tool boundary

Read-only triage. Never use `kubectl` to answer a ConfigHub-state question.

## References

- `cub-workload findings --help`.
- Companion skills: **workload-audit**, **workload-availability**, **workload-harden**.
