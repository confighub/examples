# fleet-ql — a SQL-like explorer for a ConfigHub fleet

A database-explorer SPA for **FQL** (Fleet Query Language): a SQL-like language
that queries a ConfigHub fleet — Units, the Kubernetes resources inside them,
Spaces, and Targets — and compiles down to ConfigHub's `where` / `where_data`
filters, re-checking the full predicate client-side for exact results.

```sql
SELECT unit, `spec.template.spec.containers.*.image` AS image
FROM resources
WHERE kind = 'Deployment'
  AND `spec.template.spec.containers.*.image` LIKE '%:latest'
```

## What's here

- **`src/fql/`** — the portable, dependency-free query engine (lexer → parser →
  planner → executor) plus its vitest suite. This is the language; it has no
  React or app coupling. See [`src/fql/README.md`](src/fql/README.md) for the
  grammar and the pushdown/soundness model.
- **`src/pages/ExplorerPage.tsx`** — the explorer UI: a schema sidebar (the
  virtual tables and their columns), an FQL editor with autocomplete, a
  "show plan" view of the compiled API calls, and a results grid.
- **`src/api/`** — `fqlTransport` (the engine's `Transport` over ConfigHub's
  REST API) and a minimal token/auth helper. No generated SDK.

## Run it

```bash
npm install
CONFIGHUB_URL=https://hub.confighub.com npm run dev   # http://localhost:5190
```

Paste a token from `cub auth get-token` when prompted (a same-origin deployment
uses the session cookie instead). The dev server proxies `/api` to
`CONFIGHUB_URL`.

```bash
npm test          # the FQL engine suite (vitest)
npm run build     # tsc + production build
```

## Tables (v1)

| Table | What it queries |
|---|---|
| `units` | ConfigHub Units — slug, space, toolchain, revision/drift fields, gates, labels |
| `resources` | the Kubernetes resources inside Units (all kinds) — `kind`, `name`, raw YAML paths, annotations |
| `spaces` | Spaces — slug, labels, annotations |
| `revisions` | per-Unit change history — `revisionNum`, `source`, `description`, `createdAt`, scoped by `unit`/`space` |
| `grants` | effective RBAC access — "who can VERB RESOURCE, on which cluster" (`subject`, `cluster`, `role`, `scope`, …) |
| `roles` | Role/ClusterRole inventory — `hasWildcard`, `aggregated`, `ruleCount`, `labels.*` |
| `bindings` | RoleBinding/ClusterRoleBinding inventory — `roleRef`, `subjectCount`, `orphaned`, `clusterAdmin` |
| `rbac_findings` | RBAC hygiene findings (`analyzeFleet`) — `analyzer`, `severity`, `cluster`, `resourceName`, `message` |

`events`, `triggers`, `filters`, and `links` parse today but aren't wired to the
planner yet — see the engine README.
