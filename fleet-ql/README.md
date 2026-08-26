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
- **`src/api/`** — `fqlTransport`, the engine's `Transport` over ConfigHub's REST
  API, built on the published typed client
  ([`@confighub/api`](https://www.npmjs.com/package/@confighub/api)). The auth
  shell and the RBAC engine it queries are shared with the other example
  consoles in [`../webkit`](../webkit).

## Run it

Register the app once to get an OAuth `client_id` (public, not a secret — it registers
in whatever organization your `cub` is logged into, and the app signs users into that
organization only):

```bash
cub oauthclient create fleet-ql --redirect-uri http://localhost:5190/
cp .env.example .env      # paste the client_id into VITE_OAUTH_CLIENT_ID
```

```bash
npm install
npm run dev               # http://localhost:5190
```

Sign in with the Log in button: auth is the browser-direct OIDC PKCE flow run by
[`@confighub/react-auth`](https://github.com/confighub/js-sdk), so there is no proxy to
stand up and no token to paste. The port is pinned because it has to match the
registered `redirect-uri`.

```bash
npm test          # the FQL engine suite (vitest)
npm run build     # tsc + production build
```

## Tables (v1)

| Table | What it queries |
|---|---|
| `units` | ConfigHub Units — slug, space, `cluster`, `environment`/`component`/`region`, revision/drift fields, gates, labels |
| `resources` | the Kubernetes resources inside Units (all kinds) — `kind`, `name`, `cluster`, `environment`/`component`/`region`, raw YAML paths, annotations |
| `spaces` | Spaces — slug, labels, annotations |
| `revisions` | per-Unit change history — `revisionNum`, `source`, `description`, `createdAt`, scoped by `unit`/`space` |
| `grants` | effective RBAC access — "who can VERB RESOURCE, on which cluster" (`subject`, `cluster`, `role`, `scope`, …) |
| `roles` | Role/ClusterRole inventory — `hasWildcard`, `aggregated`, `ruleCount`, `labels.*` |
| `bindings` | RoleBinding/ClusterRoleBinding inventory — `roleRef`, `subjectCount`, `orphaned`, `clusterAdmin` |
| `rbac_findings` | RBAC hygiene findings (`analyzeFleet`) — `analyzer`, `severity`, `cluster`, `resourceName`, `message` |

`events`, `triggers`, `filters`, and `links` parse today but aren't wired to the
planner yet — see the engine README.

## Demo fleet

`scripts/fleet-setup.sh` seeds a multi-component dev/staging/prod fleet
(`acme-storefront/orders/payments` × 3 envs, bound to env-clusters, labeled with
the well-known fleet labels) with a `dev → staging → prod` promotion chain.
`scripts/live.ts` runs read-only fleet + promotion scenarios against a live
ConfigHub (`npx vite-node scripts/live.ts`); `scripts/fleet-teardown.sh` removes it.
