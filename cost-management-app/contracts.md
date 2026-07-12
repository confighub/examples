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

### `node cli.mjs approve --grant ...` / `node cli.mjs commit --approval <id>`

- approve --grant mutates: local only (writes a single-use approval under
  `data/approvals/`, pinned to the Unit's head revision at grant time)
- commit mutates: yes (ConfigHub) — exactly the approved scope, nothing else
- stable refusal reasons, all `BLOCK` at exit 0: `APPROVAL_REQUIRED`,
  `APPROVAL_NOT_FOUND`, `APPROVAL_ALREADY_CONSUMED`, `FUNCTION_NOT_ALLOWED`,
  `FUNCTION_ARGS_INVALID`, `APPROVAL_REVISION_DRIFT`, `MUTATION_SILENT_SKIP`
- stable success: `MUTATION_COMMITTED` with `revisionBefore`,
  `revisionAfter`, and a receipt path under `data/receipts/`
- proves: the write path exists and functions, and every unsafe shape is a
  typed refusal — including the silent-skip class, where a mutation that
  reports success without creating a new revision is treated as a failure
- whitelist: `set-replicas`, `set-container-resources-defaults` — closed;
  entries are earned by receipted live executions, never added by
  configuration

### `npm run binding:check`

- mutates: no
- output shape: JSON with `verdict` and `reason`
- stable states: `WATCH` / `LIVE_BINDINGS_MISSING` on a cold clone; `WATCH` /
  `LIVE_BINDINGS_BLOCKED` when read surfaces are bound but write-side
  bindings carry explicit `blocked:` reasons; `PASS` only when every binding
  is real
- proves: read surfaces and write gates are tracked separately and honestly

### `npm run verify`

- mutates: no
- output shape: plain text; exit 0 on success
- proves: content hygiene plus the full test suite, including the engine
  honesty rules and executor refusals, on a cold clone with no dependencies
