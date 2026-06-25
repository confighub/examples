# Vendored: rbac engine

`model.ts`, `semantics.ts`, `whocan.ts`, `fixtures.ts` are copied from
`../../../rbac-manager/app/src/rbac/`. They are pure, dependency-free functions
(parse RBAC docs → cluster snapshots → effective-access queries), reused here to
back FQL's `grants` virtual table. fleet-ql and rbac-manager are separate Vite
apps, so a cross-app import isn't viable — hence the copy.

Local additions (keep when re-syncing):
- `semantics.ts`: `PartialAccessQuery` + `accessMatches()` — a wildcard-tolerant
  variant of `ruleMatches` for partial WHERE clauses.
- `grants.ts`: FQL-specific glue (`materializeGrants`) — NOT vendored; owned here.

To re-sync the vendored files, re-copy from rbac-manager and re-apply the
`accessMatches` addition.
