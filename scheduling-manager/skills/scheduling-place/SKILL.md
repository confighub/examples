---
name: scheduling-place
description: 'Set Kubernetes workload placement as config-as-data with the cub-scheduling CLI — nodeSelector, tolerations, and node affinity — one workload or across a whole selector via reusable placement profiles. Use for "pin this workload to the gpu pool", "add a toleration for the spot taint", "put the ml deployments on the gpu nodes", "set node affinity to a zone", "apply the placement-gpu profile", "roll placement downstream". Dry-run by default; requires --commit --change-desc. Not for read-only checks (use scheduling-audit / scheduling-findings), enforcement Triggers (use scheduling-guardrails), or pod anti-affinity / topology spread (that is availability, owned by workload-manager).'
phase: act
allowed-tools: Bash(cub-scheduling --help) Bash(cub-scheduling * --help) Bash(cub auth status) Bash(cub-scheduling preflight) Bash(cub-scheduling placement *) Bash(cub-scheduling findings *) Bash(cub-scheduling set-node-selector *) Bash(cub-scheduling set-tolerations *) Bash(cub-scheduling set-node-affinity *) Bash(cub-scheduling profile) Bash(cub-scheduling profile *) Bash(cub-scheduling fleet-edit *) Bash(cub-scheduling promote *)
---

# scheduling-place

Set where workloads land — as data, dry-run by default. One workload, across a `--where` selector via a placement profile, or promoted downstream.

- **`set-node-selector <space>/<unit> --selector k=v`** — pin to matching nodes.
- **`set-tolerations <space>/<unit> --toleration key[=value][:effect]`** — tolerate node taints.
- **`set-node-affinity <space>/<unit> --required "key=v1,v2"`** — a required node affinity term (operator In).
- **`profile install | list | apply`** — the placement profile library (stored Invocations): `placement-gpu`, `placement-spot`, and parameterized `node-pool` (`--param pool=…`).
- **`fleet-edit --profile <slug> [--where …]`** — apply a profile across a selector in one operation.
- **`promote`** — override-preserving upgrade of downstream Units.

All **edit Units but do not apply them** to a cluster — rolling out is a separate `cub unit apply`.

## Why this matters

A toleration only *permits* scheduling onto a tainted node — pair it with a nodeSelector or node affinity to actually land there. `cub-scheduling` edits the source of record with `set-yq` under the hood; everything is **dry-run by default** and requires `--commit --change-desc`, and never bypasses ApplyGates.

## When to use

- "Pin the trainer to the gpu pool." → `set-node-selector` or `profile apply placement-gpu`.
- "Tolerate the spot taint." → `set-tolerations … --toleration spot:NoSchedule`.
- "Put it on a specific zone." → `set-node-affinity … --required "topology.kubernetes.io/zone=us-east-1a,us-east-1b"`.
- "Put every prod ml workload on gpu nodes." → `fleet-edit --profile placement-gpu --environment prod --component ml`.
- "Pin to an arbitrary pool." → `profile apply node-pool <space>/<unit> --param pool=<name>`.
- "Roll the placement change downstream." → `promote --component <c>`.

## Do not load for

- Read-only checks — use **scheduling-audit** / **scheduling-findings**.
- Enforcement Triggers — use **scheduling-guardrails**.
- Pod anti-affinity / topology spread — use **workload-manager** (availability).
- Applying Units to a cluster — that is `cub unit apply`.

## Preflight gates

1. `cub-scheduling preflight` succeeds. If it fails, ask the user to run `cub auth login` (interactive) and retry.
2. For `profile apply` / `fleet-edit`, the library exists — run `cub-scheduling profile install` once if `profile list` is empty.
3. The user has write permission on the target Space(s).

## The loop

1. **See the gap**: `cub-scheduling placement --unconstrained-only` / `cub-scheduling findings`.
2. **Preview** (dry-run — the default):
   ```bash
   cub-scheduling set-node-selector ml-prod/trainer --selector pool=gpu
   cub-scheduling fleet-edit --profile placement-gpu --environment prod --component ml
   ```
3. **Commit** with `--commit --change-desc` (summary + verbatim user prompt).
4. **Verify**: `cub-scheduling placement --cluster <c> --namespace <ns>` shows the workload constrained; `findings` clears.
5. **Roll out** is a separate step — hand off to `cub-apply`.

## Safety

- A nodeSelector or required node affinity that no node satisfies leaves pods `Pending`. Confirm the pool / zone exists before committing broadly.
- Prefer scoping `fleet-edit` narrowly (dry-run shows the count) before a fleet-wide commit.

## Stop conditions

- An ApplyGate attaches (a validating Trigger failed). **Do not bypass** — fix the data (or the rule), via **triggers-and-applygates**.
- The user wants to apply to a cluster — hand off to `cub-apply`.

## Tool boundary

Allowed: `set-node-selector`, `set-tolerations`, `set-node-affinity`, `profile`, `fleet-edit`, `promote` (dry-run by default; `--commit` passes `--change-desc`), and the read commands. Not allowed: bypassing gates, `kubectl` mutations, applying to clusters.

## References

- `cub-scheduling set-node-selector --help`, `… set-tolerations --help`, `… set-node-affinity --help`, `… profile --help`, `… fleet-edit --help`, `… promote --help`.
- Companion skills: **scheduling-audit**, **scheduling-findings**, `promote-release`, `triggers-and-applygates`, `cub-apply`.
