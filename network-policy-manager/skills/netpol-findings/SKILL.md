---
name: netpol-findings
description: 'Run NetworkPolicy hygiene and anti-pattern checks across a ConfigHub fleet with the cub-netpol CLI. Use for "audit our NetworkPolicy hygiene", "any namespaces missing a default-deny?", "any workloads uncovered on ingress?", "any allow-all policies?", "any egress to the cloud metadata IP?", "any ingress/egress asymmetry?". Reports findings (missing-default-deny-ingress, uncovered-ingress, allow-all, metadata-egress, ingress-egress-asymmetry) with severity. Not for plain inventory/coverage (use netpol-audit) or connectivity (use netpol-connectivity); read-only — fixing is netpol-fix.'
phase: verify
allowed-tools: Bash(cub-netpol --help) Bash(cub-netpol * --help) Bash(cub auth status) Bash(cub-netpol preflight) Bash(cub-netpol findings) Bash(cub-netpol findings *)
---

# netpol-findings

Run the NetworkPolicy analyzer set over the fleet and report hygiene/anti-pattern findings with severity. Read-only.

## Why this matters

These are set-aware checks a per-resource validator can't make: whether a namespace lacks a default-deny, whether a workload is uncovered, and whether two policies disagree (an egress allow with no matching ingress allow). `cub-netpol` computes them over the connectivity model and returns structured findings.

## The analyzer set

| Analyzer | Severity | What it flags |
| --- | --- | --- |
| `missing-default-deny-ingress` | high | a namespace with workloads but no default-deny ingress |
| `uncovered-ingress` | high | a workload selected by no ingress NetworkPolicy |
| `allow-all` | medium | a policy rule with an empty `from`/`to` (admits all peers) |
| `metadata-egress` | high | an egress ipBlock that admits the cloud-metadata IP 169.254.169.254 |
| `ingress-egress-asymmetry` | medium | one side allows a flow the other side silently drops |

## When to use

- "Audit our NetworkPolicy hygiene." / "what's wrong with our network policies?"
- "Any namespaces missing a default-deny?" / "any uncovered workloads?"
- "Any allow-all policies / egress to the metadata IP / ingress-egress mismatches?"

## Do not load for

- Plain inventory or coverage counts — use **netpol-audit** (`coverage` overlaps but is the per-namespace table; `findings` is the prioritized issue list).
- Reachability — use **netpol-connectivity**.
- Fixing anything — use **netpol-fix** (this skill only reports).

## Preflight gates

1. `cub-netpol preflight` succeeds. If not, ask the user to run `cub auth login` and retry.

## The toolkit

```bash
cub-netpol findings -o table
cub-netpol findings --severity high
cub-netpol findings --analyzer uncovered-ingress
cub-netpol findings --cluster prod-cluster
```

JSON is the default; the `totals` block gives counts by severity and analyzer. Filter server-side with `--severity`, `--analyzer`, `--cluster`.

## Acting on findings

Report the findings grouped by severity, each with its namespace/resource and message. Then route remediation:

- `missing-default-deny-ingress` / `uncovered-ingress` → **netpol-fix** `default-deny` (or **netpol-fleet** `fleet default-deny` for the whole fleet).
- `metadata-egress` → **netpol-fix** `fix metadata`.
- `allow-all` → **netpol-fix** (replace the empty `from`/`to` with explicit peers) — or generate them from Links via `allow-from-links`.
- `ingress-egress-asymmetry` → add the missing side with **netpol-fix** `allow`.

To make findings **enforced** (advisory ApplyWarnings) rather than just reported, see **netpol-guardrails**.

## Stop conditions

- No findings — say so plainly (the fleet is clean on the v1 checks).
- The user wants the fix applied — hand off to **netpol-fix** / **netpol-fleet**.

## Tool boundary

Read-only. Never mutate, never `kubectl`.

## References

- `cub-netpol findings --help`.
- Companion skills: **netpol-audit**, **netpol-fix**, **netpol-guardrails**.
