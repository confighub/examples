# FQL — Fleet Query Language

A small, dependency-free, SQL-like query language for a ConfigHub fleet. You
write SQL; FQL compiles the parts it can to ConfigHub's `where` / `where_data` /
`whereResource` filters, runs those, and re-evaluates the full query client-side
for an exact answer. The engine is portable TypeScript (no React, no runtime
deps) — the app wires it up via a `Transport` adapter.

```ts
import { runQuery } from './fql';
const res = await runQuery(
  "SELECT unit FROM resources WHERE kind = 'Deployment' " +
    "AND `spec.template.spec.containers.*.image` LIKE '%:latest'",
  transport,
);
```

## Grammar (v1)

```
SELECT  proj [, proj]*            -- or *
FROM    table [AS? alias]
        [ [INNER | LEFT [OUTER]] JOIN table [AS? alias] ON eqs ]   -- one join (two tables)
[WHERE  expr]
[GROUP BY col [, col]*]
[ORDER BY (col|agg) [ASC|DESC] [, ...]]
[LIMIT  int]

proj := (col | AGG(col|*)) [AS alias]
AGG  := COUNT | MAX | MIN | SUM | AVG
expr := expr OR expr | expr AND expr | NOT expr | ( expr ) | predicate
predicate :=
    col <op> value
  | col [NOT] IN (v, ...)
  | col [NOT] LIKE 'pat' | col ILIKE 'pat'
  | col IS [NOT] NULL
op := = != < > <= >= ~ ~* !~ !~*      -- ~ family = POSIX regex
col := name | name.dotted.path | `backtick quoted path` | alias.<any of those>
value := string | number | TRUE | FALSE | now() [ (+|-) [interval] 'DUR' ]
```

Strings use single quotes with `''` escaping (`'it''s'`). `--` starts a line
comment. Keywords are case-insensitive; identifiers are not.

**`now()` and intervals.** `now()` folds to the current instant; `now() - interval
'24h'` shifts it. This is the "what changed recently" idiom against the `revisions`
table's `createdAt`:

```sql
SELECT unit, revisionNum, userId FROM revisions
WHERE space = 'sec-demo-dev' AND createdAt > now() - interval '24h'
ORDER BY createdAt DESC
```

The `interval` keyword is optional (`now() - '7d'` works). Duration units are
`s`/`m`/`h`/`d`/`w` (with long forms like `'3 hours'`, `'1 day'`); months and years
are rejected because they aren't fixed-length. The folding happens at **parse
time** to a constant RFC3339-UTC string literal, so the predicate pushes down to
`where` exactly like a hand-typed timestamp — and because RFC3339-UTC sorts
lexically, the server's timestamp compare and the client-side string compare agree.

**Columns** come in three flavors:

- **Curated** — a small set of generic, single-valued shorthands (`slug`,
  `space`, `kind`, `name`, `namespace`, `replicas`, …), some pushed to `where`,
  some to `where_data` (see table below). These are NOT domain fields: there is
  no `image` or `severity` column — `image` was a lossy join over a container
  array and severity is a sec-scanner annotation. Query those as the real paths
  they are (see Raw YAML data paths below).
- **Map keys** — `labels.env`, `annotations.<key>` read entity maps.
- **Raw YAML data paths** (`resources` only) — any other dotted path is treated
  as a path into the resource document. Use one of two forms for exotic
  segments:
  - **Bracket subscript** for a key that contains dots/slashes (the common case
    for annotations/labels) — the key is one atomic segment and is **not**
    re-split: `metadata.annotations['sec-scanner.confighub.com/max-severity']`,
    `metadata.labels['app.kubernetes.io/name']`. Array indices too:
    `spec.containers[0].image`. This is the same subscript form the gate's CEL
    uses (`r.metadata.annotations['…']`), so queries and policy read alike.
  - **Backtick-quoting** for a path with `*` wildcards:
    `` `spec.template.spec.containers.*.image` ``. `*` matches **any** array
    element (existential: true if any element matches); `[0]` indexes one.

  A clean path (`spec.replicas`, `kind`, `spec.containers[0].image`) pushes down
  to `where_data`; a path whose key contains dots/slashes can't be expressed in
  ConfigHub's dotted `where_data`, so it's evaluated client-side (the residual
  filter always runs, so the result is identical — just less server narrowing).

**The cluster dimension.** A Unit deploys to a **cluster** = its Target's slug,
falling back to the Space slug for unbound ("paper cluster") Units. `units` and
`resources` both expose `cluster` (and the raw `target`). This is the "which
cluster" axis — group or filter a fleet by where it actually runs, across Spaces:

```sql
SELECT cluster, COUNT(*) AS units FROM units GROUP BY cluster
SELECT unit, name FROM resources WHERE cluster = 'prod' AND kind = 'ClusterRole'
```

Because cluster's Space fallback can't be expressed as one sound pushdown clause
(`Target.Slug = x` would miss unbound Units in Space `x`), `cluster` is filtered
client-side; use `target` or `space` directly when you want server-side narrowing.

The `resources` table is **all-kinds**: every Kubernetes resource in each Unit
(Deployment, Service, ConfigMap, Ingress, …), not just Deployments. Narrow with
`WHERE kind = 'Service'` or `WHERE resourceType = 'apps/v1/Deployment'`. Curated
columns like `image`/`replicas` are Deployment-shaped sugar and read null on
kinds that lack those paths.

A **table alias** (`FROM resources r`) qualifies columns as `r.col` — ergonomic
for single-table queries and required for joins (see Joins). Curated columns are
just sugar over common raw paths; drop to a backtick path for anything not curated.

**Column-to-column comparison.** The right-hand side of a comparison can be
another column, which is how you express ConfigHub's drift idioms:

```sql
SELECT slug FROM units WHERE headRevisionNum > liveRevisionNum   -- unapplied changes
SELECT slug FROM units WHERE liveRevisionNum = 0                  -- never applied
SELECT slug FROM units WHERE upstreamRevisionNum > 0             -- clones
```

**Keyword field names.** FQL columns are camelCase, but the parser still accepts
identifiers that collide with SQL keywords wherever a column is expected — e.g.
ConfigHub's `From` field on filters, or a capitalized `Source` — so raw
ConfigHub attribute names remain usable in hand-written queries.

### Drift / gate / audit examples

```sql
-- units with unapplied changes, per space
SELECT space, COUNT(*) AS n FROM units WHERE headRevisionNum > liveRevisionNum GROUP BY space

-- what's blocked, by the exact gate key (StringBool map, pushed down)
SELECT slug FROM units WHERE applyGates['sec-demo-policy/no-critical-cves/vet-celexpr'] = true

-- ...or just by TRIGGER slug (no need to know the full key; client-side)
SELECT slug, space FROM units WHERE gate['no-critical-cves'] = true

-- ownership rollup
SELECT space, COUNT(*) AS n FROM units WHERE labels.team = 'payments' GROUP BY space
```

> Planned tables (parse today, not yet in the catalog): `events` (apply results
> / "did it deploy"), `triggers`, `filters`, `links` (dependency graph). The
> parser already accepts queries over them; only the planner/transport wiring
> remains.

## Virtual tables

| Table | Source | Notable columns |
|---|---|---|
| `units` | `GET /unit` | `slug`, `space`, `cluster`, `environment`, `component`, `region`, `toolchain`, `target`, `headRevisionNum`, `liveRevisionNum`, `lastAppliedRevisionNum`, `upstreamRevisionNum`, `upstreamUnitId`, `providerType`, `gates`, `warnings`, `labels.*`, `annotations.*`, `applyGates['<space>/<trigger>/<fn>']`, `applyWarnings[...]`, `gate['<trigger>']`, `warning['<trigger>']` |
| `resources` | `POST /function/invoke` + `get-resources` (or a revision's data blob) | `unit`, `space`, `cluster`, `target`, `environment`, `component`, `region`, `kind`, `name`, `namespace`, `replicas`, `resourceType`, `revision`, `labels.*`, + any raw data path |
| `spaces` | `GET /space` | `slug`, `displayName`, `environment`, `component`, `region`, `labels.*`, `annotations.*` |
| `revisions` | `GET /space/{id}/unit/{id}/revision` (per Unit) | `unit`, `space` (scope which units), `revisionNum`, `source`, `description`, `createdAt`, `userId` |
| `grants` | materialized from RBAC resources (`get-resources` + the rbac engine) | output: `subject`, `subjectKind`, `subjectName`, `cluster`, `space`, `unit`, `target`, `scope`, `role`, `viaBuiltin`, `binding`; access selectors: `verb`, `resource`, `apiGroup`, `namespace`, `name` |
| `roles` | materialized from RBAC resources | `name`, `kind`, `namespace`, `cluster`, `space`, `unit`, `target`, `hasWildcard`, `aggregated`, `ruleCount`, `labels.*` |
| `bindings` | materialized from RBAC resources | `name`, `kind`, `namespace`, `cluster`, `space`, `unit`, `target`, `roleRef`, `roleRefKind`, `subjectCount`, `orphaned`, `clusterAdmin` |
| `rbac_findings` | materialized from RBAC resources (`analyzeFleet`) | `analyzer`, `severity`, `cluster`, `space`, `unit`, `target`, `resourceKind`, `resourceName`, `namespace`, `message` |

`resources` has no domain columns: an image is the array path
`` `spec.template.spec.containers.*.image` `` and the scanner verdict is an
annotation, `metadata.annotations['sec-scanner.confighub.com/max-severity']`.
Both are read from the resource document and evaluated client-side (no server
index for the annotation key), so a query on them fetches broadly and filters in
the browser.

**Time travel.** `WHERE revision = N` reads each in-scope unit's resources *as of*
that revision instead of head — it fetches that revision's data blob and parses
the resources from it. `revision = 'head'` / `'live'` resolve per unit to the
unit's `headRevisionNum` / `liveRevisionNum`. The selector is a fetch parameter,
not a filter, so it's stripped from the client-side residual (and each row is
stamped with the resolved `revision`). Pair it with `unit =` / `space =` scoping.

```sql
SELECT unit, `spec.template.spec.containers.*.image` AS image
FROM resources WHERE unit = 'checkout' AND revision = 5
```

## Fleet & promotion (environment/component/region)

ConfigHub's well-known fleet labels — `Component`, `Environment`, `Region`,
`Owner`, `Variant` — organize a multi-component fleet across dev/staging/prod.
FQL exposes `environment`, `component`, and `region` as first-class columns
(sugar over `labels.Environment` etc.): on `units`/`spaces` they push to `where
Labels.<X>`; on `resources` they're stamped from the owning unit and filtered
client-side. The cluster dimension layers on top — one env-cluster (`dev-cluster`/
`staging-cluster`/`prod-cluster`) spans every component in that environment.

```sql
-- fleet inventory: every workload's version + replicas across environments
SELECT component, environment, `spec.template.spec.containers.*.image` AS image, replicas
FROM resources WHERE space LIKE 'acme-%' AND kind = 'Deployment'
ORDER BY component, environment

-- the promotion gap: dev ahead of staging/prod, awaiting promotion
SELECT environment, `spec.template.spec.containers.*.image` AS image
FROM resources WHERE component = 'storefront' AND kind = 'Deployment' ORDER BY environment

-- per-environment / per-cluster posture
SELECT environment, COUNT(*) AS units FROM units WHERE space LIKE 'acme-%' GROUP BY environment
SELECT cluster, COUNT(*) AS units FROM units WHERE space LIKE 'acme-%' GROUP BY cluster
```

Promotion itself is a ConfigHub operation (`cub unit update --upgrade` along the
`dev → staging → prod` upstream chain, or `cub variant promote`); FQL is how you
*see* the gap before and confirm convergence after. `scripts/fleet-setup.sh`
seeds this fleet; `scripts/live.ts` runs the scenarios end to end.

## `grants` — who can do what, on which cluster

The `grants` table answers effective-access questions across the fleet. It is
**materialized client-side**: the transport fetches the in-scope units' RBAC
resources and the rbac engine resolves bindings → roleRefs → roles → rules
(expanding `*`, unioning aggregated ClusterRoles, honoring `cluster-admin`) —
something no `where_data` path can express. One row per resolved
(cluster, binding, subject).

```sql
-- who can delete pods, fleet-wide, on which cluster, granted by what
SELECT subject, cluster, role, scope FROM grants
WHERE verb = 'delete' AND resource = 'pods' ORDER BY cluster

-- the inverse (subject) view: everything a group holds, where
SELECT cluster, role, scope FROM grants WHERE subject = 'Group:developers'

-- blast radius of secret access (effective, incl. via wildcard / cluster-admin)
SELECT subject, cluster FROM grants WHERE resource = 'secrets' AND verb = 'get'

-- who can exec into prod pods, specifically
SELECT subject, cluster FROM grants
WHERE resource = 'pods/exec' AND verb = 'create' AND cluster = 'prod'

-- superuser holders across the fleet
SELECT subject, cluster, binding FROM grants WHERE role = 'cluster-admin'
```

**Access selectors vs output columns.** `verb`, `resource`, `apiGroup`,
`namespace`, and `name` are the *access question* — they drive the materializer's
RBAC matching (so `verb = 'delete'` also matches a rule with `verbs: ['*']`), and
are **stripped from the client-side residual** exactly like the `revision`
selector. Omit one for "any" (`resource = 'pods'` with no verb = who can do
anything to pods). Everything else (`subject`, `cluster`, `role`, `scope`,
`viaBuiltin`, …) is a real output column, filtered and projected client-side — so
`WHERE subject = 'Group:developers'` is the inverse "what does this subject hold"
view, and `cluster = 'prod'` narrows by where it runs. `space` pushes down to
narrow the RBAC fetch; `cluster` is client-side (Target/Space fallback).

## `roles` / `bindings` — the structural RBAC inventory

Where `grants` answers *effective* access, `roles` and `bindings` are the
**object inventory** — one row per Role/ClusterRole and per RoleBinding/
ClusterRoleBinding, materialized from the same RBAC fetch with computed audit
flags (the same logic as the rbac findings analyzers).

```sql
-- wildcard roles, fleet-wide
SELECT cluster, name, kind FROM roles WHERE hasWildcard = true

-- aggregated ClusterRoles
SELECT cluster, name FROM roles WHERE aggregated = true

-- orphaned bindings (roleRef points at a role that doesn't exist, not a builtin)
SELECT cluster, name, roleRef FROM bindings WHERE orphaned = true

-- every superuser binding across the fleet
SELECT cluster, name, roleRef FROM bindings WHERE clusterAdmin = true

-- roles owned by a team, by cluster
SELECT cluster, name FROM roles WHERE labels.team = 'platform'
```

`space` pushes down to narrow the RBAC fetch; everything else (including
`cluster`, and the computed flags `hasWildcard` / `aggregated` / `orphaned` /
`clusterAdmin`) is filtered client-side.

## `rbac_findings` — the audit complement

`rbac_findings` runs every RBAC hygiene analyzer (`analyzeFleet`) and yields one
row per finding — the analysis the boolean flags on `roles`/`bindings` don't
cover: escalation verbs (`escalate`/`bind`/`impersonate`), risky grants
(secrets, `pods/exec`, webhooks/CRDs), and unbound ServiceAccounts, alongside
wildcards, cluster-admin, and orphaned bindings.

```sql
-- the high-severity audit queue, by cluster
SELECT cluster, analyzer, resourceName, message
FROM rbac_findings WHERE severity = 'high' ORDER BY cluster

-- finding counts by analyzer
SELECT analyzer, COUNT(*) AS n FROM rbac_findings GROUP BY analyzer ORDER BY n DESC

-- who can escalate privileges, where
SELECT cluster, resourceName FROM rbac_findings WHERE analyzer = 'privilege-escalation-verbs'
```

Analyzers: `wildcard-rules`, `privilege-escalation-verbs`, `risky-grants`,
`cluster-admin-bindings`, `orphaned-bindings`, `unbound-service-accounts`.

## Joins

FQL supports a **two-table equi-join** (`INNER` or `LEFT`), including a
self-join. ConfigHub has no server-side join, so it's done **client-side**: each
side is fetched independently (its single-alias WHERE predicates push down), then
the rows are hash-joined on the `ON` equalities and the full WHERE is re-checked
over the combined rows. The flagship use is the cross-environment / cross-revision
diff:

```sql
-- which components' dev image differs from prod (the promotion gap)
SELECT d.component AS component,
  `d.spec.template.spec.containers.*.image` AS dev,
  `p.spec.template.spec.containers.*.image` AS prod
FROM resources d JOIN resources p ON d.name = p.name AND d.kind = p.kind
WHERE d.space LIKE 'acme-%' AND p.space LIKE 'acme-%'
  AND d.environment = 'Dev' AND p.environment = 'Prod' AND d.kind = 'Deployment'
  AND `d.spec.template.spec.containers.*.image` != `p.spec.template.spec.containers.*.image`

-- units with no resources (LEFT join + null right side)
SELECT u.slug FROM units u LEFT JOIN resources r ON u.slug = r.unit
```

Rules: **both tables need aliases**; **every column must be alias-qualified**
(`d.kind`); the `ON` is an AND of `a.col = b.col` equalities on plain columns
(not raw paths). A **qualified raw path** puts the alias *inside* the backticks —
`` `d.spec.template…image` `` — because `d.` outside can't be lexed before a
backtick. Fetch selectors (`revision`, the grants access-query) aren't supported
inside a join (v1). Deferred: 3+ tables, non-equi joins, and joining the
materialized RBAC tables.

## How it executes (the pushdown model)

ConfigHub's filter grammar is **AND-only — no OR, no parentheses**. FQL bridges
the gap:

1. **Parse** → AST (hand-written lexer + recursive-descent/precedence parser).
2. **Plan** → bind & type-check columns, normalize WHERE to **DNF** (an OR of
   AND-groups), and for each group emit the pushable predicates as ConfigHub
   clause strings. A top-level `OR` becomes **one API call per branch**.
3. **Execute** → run the per-group fetches concurrently, **union + de-dupe** the
   rows.
4. **Evaluate** → re-run the *complete* original WHERE client-side, then
   project / aggregate / order / limit.

> **Invariant:** pushdown only narrows the fetch; the full WHERE is always
> re-checked client-side. Correctness never depends on how much got pushed down.

"Show plan" in the console surfaces exactly which calls a query compiles to.

## Module map

```
lexer.ts     source → tokens (+ positions)
parser.ts    tokens → AST (ast.ts), precedence: OR < AND < NOT < comparison
schema.ts    virtual-table catalog: column → type + pushdown target/expr
planner.ts   AST → ExecutionPlan (NNF, DNF, pushdown partition, validation)
compile.ts   predicate → ConfigHub clause string (escaping, IN-value gate)
evaluate.ts  client-side filter / project / group+agg / order / limit
transport.ts the Transport seam (mock in tests, fqlTransport in the app)
executor.ts  run plan → fetch per group → union/dedup → evaluate
index.ts     public API: parse / planQuery / runQuery
```

Tests live in `__tests__/` (vitest): `npm test`.

## Out of scope (v2)

Joins beyond two tables / non-equi joins / joining the materialized RBAC tables,
DISTINCT, subqueries, and any write/DML — FQL is read-only; mutations stay with
the existing server-side `yq-i` path. Saved queries / history are a UI concern,
not the language.
