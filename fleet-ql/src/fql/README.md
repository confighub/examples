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
FROM    (units | resources | spaces | revisions) [AS? alias]
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
```

Strings use single quotes with `''` escaping (`'it''s'`). `--` starts a line
comment. Keywords are case-insensitive; identifiers are not.

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

The `resources` table is **all-kinds**: every Kubernetes resource in each Unit
(Deployment, Service, ConfigMap, Ingress, …), not just Deployments. Narrow with
`WHERE kind = 'Service'` or `WHERE resourceType = 'apps/v1/Deployment'`. Curated
columns like `image`/`replicas` are Deployment-shaped sugar and read null on
kinds that lack those paths.

A **table alias** (`FROM resources r`) qualifies columns as `r.col` — purely
ergonomic in v1 (no JOINs yet). Curated columns like `image` are just sugar over
common raw paths; drop to a backtick path for anything not curated.

**Column-to-column comparison.** The right-hand side of a comparison can be
another column, which is how you express ConfigHub's drift idioms:

```sql
SELECT slug FROM units WHERE HeadRevisionNum > LiveRevisionNum   -- unapplied changes
SELECT slug FROM units WHERE LiveRevisionNum = 0                  -- never applied
SELECT slug FROM units WHERE UpstreamRevisionNum > 0             -- clones
```

**Keyword field names.** ConfigHub has fields that collide with SQL keywords
(`From` on filters, `Source` on revisions). These are accepted as column names
wherever a column is expected.

### Drift / gate / audit examples

```sql
-- units with unapplied changes, per space
SELECT space, COUNT(*) AS n FROM units WHERE HeadRevisionNum > LiveRevisionNum GROUP BY space

-- what's blocked, and by which gate (StringBool map, pushed down)
SELECT slug FROM units WHERE ApplyGates['sec-demo-policy/no-critical-cves/vet-celexpr'] = true

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
| `units` | `GET /unit` | `slug`, `space`, `toolchain`, `target`, `headRev`/`HeadRevisionNum`, `LiveRevisionNum`, `LastAppliedRevisionNum`, `UpstreamRevisionNum`, `UpstreamUnitID`, `ProviderType`, `gates`, `warnings`, `labels.*`, `annotations.*`, `ApplyGates['<space>/<trigger>/<fn>']`, `ApplyWarnings[...]` |
| `resources` | `POST /function/invoke` + `get-resources` | `unit`, `space`, `kind`, `name`, `namespace`, `replicas`, `resourceType`, `labels.*`, + any raw data path |
| `spaces` | `GET /space` | `slug`, `displayName`, `labels.*`, `annotations.*` |
| `revisions` | `GET /space/{id}/unit/{id}/revision` (per Unit) | `unit`, `space` (scope which units), `RevisionNum`, `Source`, `Description`, `CreatedAt`, `UserID` |

`resources` has no domain columns: an image is the array path
`` `spec.template.spec.containers.*.image` `` and the scanner verdict is an
annotation, `metadata.annotations['sec-scanner.confighub.com/max-severity']`.
Both are read from the resource document and evaluated client-side (no server
index for the annotation key), so a query on them fetches broadly and filters in
the browser.

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

JOINs across tables, DISTINCT, subqueries, and any write/DML — FQL is read-only;
mutations stay with the existing server-side `yq-i` path. Saved queries / history
are a UI concern, not the language.
