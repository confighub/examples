# cub-netpol — NetworkPolicy manager for ConfigHub

`cub-netpol` manages Kubernetes **NetworkPolicy** config stored as data in
ConfigHub, reasoning about it together with the Namespaces, workloads, and
Services it must cover — across a whole fleet of cluster-Spaces. It is designed
for use by an AI agent in a terminal, and is a sibling of
[`rbac-manager-for-agents`](../rbac-manager-for-agents).

**Why a manager, not just a validator?** A per-resource policy check (kube-score,
Kyverno, OPA) only ever sees one object at a time, so it can't answer the
questions that actually matter for network segmentation — *does every namespace
have a default-deny? is this workload uncovered on egress? does an egress allow
have a matching ingress allow?* Those are properties of the **whole set** of
resources. And because such checks only report, fixing a finding means editing
the cluster out-of-band, which causes drift. `cub-netpol` reasons over the whole
ConfigHub-managed set and fixes issues **as data**, so the cluster converges
through the normal apply pipeline with no drift.

## Commands

All read commands default to JSON (`-o json`); pass `-o table` for a human view.
All write commands are **dry-run by default** and require `--commit` with a
`--change-desc`; they create/edit Units but do **not** apply them to a cluster
(rolling out is a separate, deliberate `cub unit apply`).

| Command | Kind | What it does |
|---|---|---|
| `preflight` / `version` | diag | Verify the ConfigHub session; print version |
| `snapshot` | read | Per-cluster inventory: NetworkPolicy / namespace / workload / service + gated/unapplied Units |
| `list` | read | Enumerate NetworkPolicy-relevant resources across the fleet |
| `coverage` | read | Per-namespace default-deny presence + uncovered workloads (the coverage gap report) |
| `who-can-reach` / `reachable-from` | read | Effective connectivity under the policy set |
| `findings` | read | Hygiene & anti-pattern checks (missing-default-deny, uncovered, allow-all, metadata-egress, asymmetry) |
| `default-deny <ns>` | write | Author a default-deny NetworkPolicy Unit for a namespace (`--egress` adds egress + DNS) |
| `allow <src> <dst>` | write | Author an allow policy between two workloads |
| `allow-from-links` | write | Derive allow policies from ConfigHub **Links** (the dependency graph); consolidates per destination, upserts on re-run |
| `fix dns\|metadata <space>/<unit>` | write | Patch an existing policy (add DNS egress / except the cloud-metadata IP) via `set-yq` |
| `fleet default-deny` | write | Bulk: a default-deny for **every** uncovered namespace (idempotent) |
| `promote` | write | Override-preserving upgrade of Units behind their upstream baseline |
| `guardrails install\|status\|annotate` | write | The enforcement pack — `Warn=true` `vet-celexpr` Triggers + annotate-then-validate for coverage |

## Build

```
make build      # -> bin/cub-netpol
make test
```

## Use

`cub-netpol` uses your existing `cub` session — run `cub auth login` first if you
are not signed in. It can run standalone (`cub-netpol …`) or, installed as a cub
plugin, as `cub netpol …`. ConfigHub I/O goes through the ConfigHub Go SDK
(`github.com/confighub/sdk/core/cubapi`); there is no `cub` subprocess.

```bash
# Audit
bin/cub-netpol coverage -o table
bin/cub-netpol findings --severity high -o table
bin/cub-netpol who-can-reach cartservice --cluster prod-cluster -o table

# Fix as data (dry-run, then --commit)
bin/cub-netpol fleet default-deny -o table
bin/cub-netpol fleet default-deny --commit --change-desc "Default-deny uncovered namespaces"
bin/cub-netpol allow-from-links --space apptique-prod --commit --change-desc "Allows from the dependency graph"

# Enforce
bin/cub-netpol guardrails install --where-space "Slug LIKE 'apptique-%'" --commit
bin/cub-netpol guardrails status -o table
```

A typical loop: **`coverage`/`findings`** to see the gaps → **`fleet default-deny`**
+ **`allow-from-links`** to fix them as data → **`who-can-reach`** to verify the
intended least-privilege graph → **`guardrails`** to keep it that way.

## Agent skills

`skills/` holds six ConfigHub-format agent skills (read vs. write split, scoped
`allowed-tools`, preflight gates, evals):

| Skill | Phase | Surface |
|---|---|---|
| `netpol-audit` | read | `snapshot`, `list`, `coverage` |
| `netpol-connectivity` | read | `who-can-reach`, `reachable-from` |
| `netpol-findings` | read | `findings` |
| `netpol-fix` | write | `default-deny`, `allow`, `allow-from-links`, `fix` |
| `netpol-fleet` | write | `fleet default-deny`, `promote` |
| `netpol-guardrails` | write | `guardrails install/status/annotate` |

## How it works

- **Snapshot** (`internal/snapshot`) — discovers every `Kubernetes/YAML` Unit you
  can view, runs `get-resources` server-side in parallel over NetworkPolicies,
  Namespaces, pod-bearing workloads, and Services, and joins with Unit / Space /
  Target metadata. Coverage is over ConfigHub-managed Units only.
- **Engine** (`internal/netpol`) — a deterministic, table-tested analysis package:
  label-selector matching, per-namespace/per-workload coverage, a connectivity
  model (isolation + ingress/egress rule matching, additive-OR semantics), the
  findings analyzers, and the policy-YAML generators.
- **Writes** — new Units via the SDK's unit-create; in-place edits and upserts via
  the `set-yq` function; guardrails via Triggers/Filters. Dry-run/commit and
  change descriptions throughout; nothing is applied to a cluster without a
  separate `cub unit apply`.

> Documented limitations (v1): ipBlock peers don't match pod-to-pod traffic;
> ports aren't considered in the reachability boolean; the egress default-deny
> DNS-break check and a few others are tracked as deferred analyzers.
