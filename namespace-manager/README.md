# cub-namespace — Namespace manager for ConfigHub

`cub-namespace` manages Kubernetes **namespaces and their policy envelope** —
pod-security labels, a default-deny NetworkPolicy, and baseline RBAC — stored as
data in ConfigHub across a whole fleet of cluster-Spaces. It is designed for use
by an AI agent in a terminal, and is a sibling of
[`rbac-manager-for-agents`](../rbac-manager-for-agents) and
[`network-policy-manager`](../network-policy-manager).

**Why a manager, not a runtime controller?** Tenancy controllers (Capsule, the
retired HNC) and per-resource validators each see one cluster, or one object, at
a time. They can't answer the question that matters for namespace governance —
*does every namespace in the fleet carry its full policy envelope, and is a
component's namespace consistent across all its variants?* That's a property of
the **whole set** of resources across the fleet's source of record. `cub-namespace`
reasons over the whole ConfigHub-managed set and (in later milestones) fixes gaps
**as data**, so clusters converge through the normal apply pipeline with no drift.

The cloneable envelope itself is an
[installer Package](https://github.com/confighub/installer/tree/main/packages/namespace-envelope)
(`bases/` + `set-namespace` / `set-pod-security-defaults` transformers +
`vet-placeholders` validators); `cub-namespace` owns the **fleet-wide analysis**
the installer doesn't do, and the **backfill** of the envelope into existing Spaces.

## Commands

All read commands default to JSON (`-o json`); pass `-o table` for a human view.

| Command | Kind | What it does |
|---|---|---|
| `preflight` / `version` | diag | Verify the ConfigHub session; print version |
| `snapshot` | read | Per-cluster inventory: namespace / NetworkPolicy / RBAC / workload + gated/unapplied Units |
| `list` | read | Enumerate envelope-relevant resources across the fleet |
| `envelope` | read | Per-namespace completeness — which namespaces lack pod-security / default-deny / baseline RBAC, plus duplicate-namespace-per-Target collisions |
| `consistency` | read | Per-component: is the namespace name + pod-security level identical across its variant Spaces? |
| `findings` | read | Severity-ranked governance findings (envelope gaps + duplicates + cross-variant inconsistency) |
| `apply-envelope` | write | Stamp pod-security defaults on a Space's Namespace Unit(s) (fixes `missing-pod-security`) |
| `backfill` | write | Clone a base envelope's default-deny + baseline RBAC into an existing Space, re-homed via `set-namespace` |
| `promote` | write | Override-preserving upgrade of downstream envelope Units to their upstream head |
| `guardrails install\|status\|annotate` | write | The enforcement pack — `Warn=true` `vet-celexpr` Triggers + annotate-then-validate for envelope gaps |

All write commands are **dry-run by default** and require `--commit` with a
`--change-desc`; they create/edit Units but do **not** apply them to a cluster
(rolling out is a separate, deliberate `cub unit apply`).

This build ships **M0–M4**: read-only analysis, config-as-data envelope fixes,
fleet promotion, and annotate-then-validate enforcement (guardrails). The
namespace-name invariant (`metadata.namespace == normalizeName(Component)`) is
enforced separately by a cluster-selected mutating `set-namespace` Trigger — the
promotable active-correction option, wired outside the advisory pack.

## Build

```
make build      # -> bin/cub-namespace
make test
```

## Use

`cub-namespace` uses your existing `cub` session — run `cub auth login` first if
you are not signed in. It can run standalone (`cub-namespace …`) or, installed as
a cub plugin, as `cub namespace …`. ConfigHub I/O goes through the ConfigHub Go
SDK (`github.com/confighub/sdk/core/cubapi`); there is no `cub` subprocess.

```bash
bin/cub-namespace snapshot -o table
bin/cub-namespace envelope -o table
bin/cub-namespace envelope --incomplete-only -o table       # just the gaps
bin/cub-namespace envelope --component apptique -o table    # one component, every variant
bin/cub-namespace envelope --cluster prod-cluster -o table  # one cluster (Target)
bin/cub-namespace list --kind Namespace -o table
```

## Scoping the fleet

Read commands scope the fleet **server-side** with one ConfigHub Unit `where`
predicate that may reference Unit, `Space.*`, and `Target.*` metadata (so only
the matching Units' resources are fetched):

| Flag | Compiles to |
|---|---|
| `--where "<expr>"` | raw predicate — e.g. `"Target.ProviderType = 'OCI'"` (the ProviderType recommended for ArgoCD/Flux), `"Space.Slug LIKE 'apptique-%'"` |
| `--component <v>` | `Space.Labels.Component = '<v>'` |
| `--environment <v>` | `Space.Labels.Environment = '<v>'` |
| `--region` / `--owner` / `--layer` / `--variant` `<v>` | the matching `Space.Labels.<Key>` |

The label flags are shorthands over `--where`, mirroring the standard Space
labels the `cub variant` commands use. All provided flags are `AND`-combined
(ConfigHub `where` is flat `AND`-only — **no parentheses, no `OR`**; a
parenthesized clause fails with `invalid attribute name`). With no flag,
everything you can view is in scope.

Separately, `--cluster` and `--namespace` are **client-side** display filters
that narrow the loaded results to one cluster (Target-or-Space slug) or
namespace — a presentation concern, distinct from the server-side scoping above.

## The envelope

A namespace's policy **envelope** — the members `envelope` checks for:

| Member | Present when |
|---|---|
| `namespace-object` | a `v1/Namespace` object exists for the namespace |
| `pod-security` | the Namespace carries a `pod-security.kubernetes.io/enforce` label |
| `default-deny` | a namespace-wide default-deny NetworkPolicy (empty `podSelector`) exists |
| `baseline-rbac` | a `RoleBinding` exists in the namespace |

`envelope` also flags **duplicate Namespace objects** that resolve to the same
name on the same Target (a collision in one cluster). Base Units still carrying
the `confighubplaceholder` name are exempt — `vet-placeholders` already gates
them from deploying.

## Agent skills

`skills/` holds six ConfigHub-format agent skills (read vs. write split, scoped
`allowed-tools`, preflight gates, evals):

| Skill | Kind | Surface |
|---|---|---|
| `namespace-audit` | read | `snapshot`, `list`, `envelope` |
| `namespace-consistency` | read | `consistency` |
| `namespace-findings` | read | `findings` |
| `namespace-backfill` | write | `apply-envelope`, `backfill` |
| `namespace-enforce` | write | `guardrails install/status/annotate` |
| `namespace-promote` | write | `promote` |

## How it works

- **Snapshot** (`internal/snapshot`) — discovers every `Kubernetes/YAML` Unit you
  can view, runs `get-resources` server-side in parallel over Namespaces,
  NetworkPolicies, RBAC objects, and workloads, and joins with Unit / Space /
  Target metadata. Canonical base/policy Spaces are excluded from cluster
  analysis. Clusters are ConfigHub Targets (Space slug for unbound Units).
- **Engine** (`internal/nsmanager`) — a deterministic, table-tested analysis
  package: the typed model, per-namespace envelope completeness, and the
  duplicate-namespace check.

> Clusters are ConfigHub Targets. Base/template Units are best bound to a
> **Noop** Target (a no-apply, server-hosted ProviderType), so every Unit has a
> Target and `Target.Slug` is a total grouping key and `Target.ProviderType =
> 'Noop'` cleanly distinguishes bases from deployed config.
