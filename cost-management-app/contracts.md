# Contracts: cost-management-app

Stable, machine-checkable behavior for this example. See
[EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for the
format. This is a generated operational app — a browser UI plus a
zero-dependency CLI sibling — so there is no `setup.sh`; these contracts cover
the CLI, the cost sweep, and the executor outputs that automation can assert
against.

### Result Contract (typed verdicts)

- mutates: no (classification only)
- output shape: JSON object carrying `verdict` and a stable `reason`
- applies to: every CLI loop command, `lifecycle.mjs` command,
  `npm run binding:check`, `npm run cost:sweep`, and the executor
- proves: expected blockers are successful, scriptable classifications at
  exit 0 — branch on `verdict` and `reason`, never on shell success

### `node cli.mjs findings --json`

- mutates: no
- output shape: JSON with `verdict`, `reason`, `findings[]`, and `costTotals`
  when a sweep has run
- stable fields: `findings[].code`, `findings[].severity`,
  `findings[].monthly`, `costTotals.claimedMonthlySavings`,
  `costTotals.savingsClaimRule`
- stable codes: `COST_SWEEP_NOT_RUN` before any sweep; finding rules are
  `MISSING_REQUESTS`, `NONPROD_REPLICAS`, `LEFTOVER_SPACE`, `LIMIT_HEADROOM`,
  `STACK_COPIES`
- proves: no invented numbers — without a sweep file there are no priced rows

### `npm run cost:sweep`

- mutates: no (two org-wide read-only queries; writes only the
  deployment-local `data/cost-findings.json`, which is gitignored)
- requires: a ConfigHub session that can read the org
- output shape: JSON with `verdict`, `totals`, `topFindings[]`
- stable fields: `totals.containersScanned`,
  `totals.containersMissingRequests`, `totals.configuredMonthlyRequestCost`,
  `totals.claimedMonthlySavings`, `totals.savingsClaimRule`
- honesty rules (enforced by `tests/cost-engine.test.mjs`): limits are
  exposure and never savings; a Unit with no Target binding is
  configured-cost-only and excluded from claimed savings; every priced figure
  carries the rate-card basis string

### `node cli.mjs preview --finding <id> --json`

- ConfigHub mutation: no; runs the finding-owned function with `--dry-run`
- local mutation: writes an exact preview under `data/previews/`
- binds: finding id, authenticated Cub context, internal and external org ids,
  server, Space, Unit id, Unit revision, function, arguments, expected mutation,
  and integrity digest
- stable success: `PASS` / `PREVIEW_CREATED`
- stable refusals: `FINDING_REQUIRED`, `FINDING_NOT_FOUND`,
  `FINDING_NOT_ACTIONABLE`, `FUNCTION_NOT_ALLOWED`, `FUNCTION_ARGS_INVALID`,
  `LIVE_AUTHORITY_REQUIRED`, `ORG_MISMATCH`, `PREVIEW_EMPTY`

### `node cli.mjs review --record --preview <id> --reason <why> --json`

- ConfigHub mutation: no
- local mutation: writes short-lived review evidence under `data/reviews/`
- stable success: `WATCH` / `LOCAL_REVIEW_RECORDED`
- semantics: the reviewer comes from authenticated Cub, the preview revision is
  re-read, the record expires after 15 minutes, and duplicate reviews share one
  deterministic execution idempotency key
- non-claim: this local JSON is unsigned evidence, not a ConfigHub approval
  object, authorization token, or mutation permission
- stable refusals: `PREVIEW_REQUIRED`, `PREVIEW_NOT_FOUND`, `PREVIEW_TAMPERED`,
  `AUTH_REQUIRED`, `ORG_MISMATCH`, `PREVIEW_REVISION_DRIFT`

### `node cli.mjs commit --review <id> --confirm-execute --json`

- without `--confirm-execute`: no mutation; returns `ASK` /
  `EXECUTION_CONFIRMATION_REQUIRED`
- with confirmation: mutates ConfigHub through the finding-owned, whitelisted
  function only; no hand-entered target, function, or arguments are accepted
- stable success: `PASS` / `CONFIG_REVISION_COMMITTED`, with `revisionBefore`,
  `revisionAfter`, mutation parity, and a receipt under `data/receipts/`
- local receipt class: `local-unsigned-execution-receipt`; a later local reload
  reports `WATCH` / `LOCAL_UNSIGNED_RECEIPT_RECORDED`, never signed/live proof
- stable refusals: `LOCAL_REVIEW_REQUIRED`, `REVIEW_NOT_FOUND`,
  `REVIEW_EXPIRED`, `REVIEW_ALREADY_USED`, `REVIEWER_IDENTITY_MISMATCH`,
  `REVIEW_REVISION_DRIFT`, `REVIEW_TARGET_REPLACED`,
  `REVIEW_EXECUTION_ALREADY_CLAIMED`, `CONCURRENT_REVISION_DETECTED`,
  `MUTATION_DIFF_MISMATCH`, `MUTATION_FAILED`
- proof boundary: success proves the ConfigHub revision and reviewed mutation;
  the local receipt is not a signed approval or fresh server attestation,
  provider-native expected-revision atomicity remains `WATCH`, and controller
  or runtime delivery is not claimed without separate evidence
- whitelist: `set-replicas`, `set-container-resources-defaults` — closed;
  entries are earned by receipted live executions, never added by configuration

### `npm run binding:check`

- mutates: no
- output shape: JSON with `verdict` and `reason`
- stable states: `WATCH` / `LIVE_BINDINGS_MISSING` on a cold clone;
  `LIVE_BINDINGS_MIGRATION_REQUIRED`, `LIVE_BINDINGS_PLACEHOLDER`, or
  `LIVE_BINDINGS_BLOCKED` for unusable authority; and `WATCH` /
  `LIVE_BINDINGS_REVIEW_READY` when exact review plus explicitly confirmed
  execution are bound but provider-native atomicity or delivery proof is open
- proves: review authority, execution confirmation, ConfigHub revision proof,
  and controller/runtime delivery are tracked separately; binding values alone
  never create a universal live claim

### `npm run verify`

- mutates: no
- output shape: plain text; exit 0 on success
- proves: content hygiene plus the full test suite, including the engine
  honesty rules and executor refusals, on a cold clone with no dependencies
