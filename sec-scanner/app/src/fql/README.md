# FQL — Fleet Query Language

A small, dependency-free, SQL-like query language for a ConfigHub fleet. You
write SQL; FQL compiles the parts it can to ConfigHub's `where` / `where_data` /
`whereResource` filters, runs those, and re-evaluates the full query client-side
for an exact answer. The engine is portable TypeScript (no React, no runtime
deps) — the app wires it up via a `Transport` adapter.

```ts
import { runQuery } from './fql';
const res = await runQuery(
  "SELECT unit, image FROM resources WHERE severity = 'CRITICAL'",
  transport,
);
```

## Grammar (v1)

```
SELECT  proj [, proj]*            -- or *
FROM    units | resources | spaces | targets
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
```

Strings use single quotes with `''` escaping (`'it''s'`). `--` starts a line
comment. Keywords are case-insensitive; identifiers are not. Dotted columns
(`labels.env`) read map keys.

## Virtual tables

| Table | Source | Notable columns |
|---|---|---|
| `units` | `GET /unit` | `slug`, `space`, `toolchain`, `target`, `headRev`, `gates`, `warnings`, `labels.*`, `annotations.*` |
| `resources` | `POST /function/invoke` + `get-resources` | `unit`, `space`, `kind`, `name`, `image`, `replicas`, `resourceType`, `severity`, `cveCount`, `scannedAt`, `cvedbVersion` |
| `spaces` | `GET /space` | `slug`, `displayName`, `labels.*`, `annotations.*` |
| `targets` | `GET /target` | `slug`, `displayName`, `labels.*` |

`severity`/`cveCount`/etc. on `resources` are read from the scanner's
annotations and evaluated client-side (no server index), so a query on them
fetches broadly and filters in the browser.

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
