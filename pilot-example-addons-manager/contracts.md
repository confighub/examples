# Contracts: add-on-manager

Stable, machine-checkable behavior for this example. See
[EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for the format.
This example is a generated operational app — a browser UI plus a
zero-dependency CLI sibling — not a seed-and-verify script, so there is no
`setup.sh`. These contracts cover the CLI, verification, and binding-check
outputs that automation can assert against. They are a precise view of what
`SPECIFICATION.md` and `data/operational-workflow.json` already declare; they
add no new claims.

### Result Contract (typed verdicts)

- mutates: no (classification only)
- output shape: JSON object carrying `verdict` and a stable `reason`
- applies to: the CLI loop commands documented below, every `lifecycle.mjs`
  command, and `npm run binding:check`
- verdict table (from `SPECIFICATION.md`):

| `verdict` | Meaning | Exit |
|---|---|---|
| `PASS` | The action completed. | 0 |
| `WATCH` | Not proven yet / waiting (e.g. `LIVE_BINDINGS_MISSING`). | 0 |
| `BLOCK` | Refused: the action cannot proceed as asked. | 0 |
| `ASK` | A human decision is required. | 0 |
| `ERROR` | The command could not run (malformed input, missing core file). | non-zero |

- proves: expected blockers are successful, scriptable classifications at
  exit 0 — branch on `verdict` and `reason`, never on shell success. The one
  runtime exception is `node server.mjs` after `decommission`, which refuses
  to start and exits 3.

### `npm test`

- mutates: no (no ConfigHub, Git, controller, or runtime writes; lifecycle
  tests copy the app into a temp directory first)
- output shape: node assertion run; exit 0 on success
- proves: the GUI harness, auth surface, UI tool wiring, CLI loop, and
  lifecycle commands all read the same `data/operational-workflow.json`
  contract instead of inventing their own scope or proof state.

### `npm run verify`

- mutates: no
- output shape: plain text; exit 0 on success (layout and content hygiene
  check, then `npm test`)
- proves: the package is intact on a cold clone with no dependency install.

### `node cli.mjs preflight --json`

- mutates: no
- output shape: JSON object
- stable fields: `verdict` (`PASS`|`WATCH`), `reason` (live-binding gate code,
  see below), `status`, `liveBindings`, `authMode`, `checks[]`, `nextGate`
- fresh clone: `verdict` is `WATCH` and `reason` is `LIVE_BINDINGS_MISSING`
- proves: package, auth mode, workflow contract, Variant inventory, and
  live-binding readiness are checked before any risk.

### `node cli.mjs map --json` (aliases: `list`, `snapshot`)

- mutates: no
- output shape: JSON object
- stable fields: `verdict` = `PASS`, `reason` = `VARIANT_SCOPE_MAPPED`,
  `scopeModel`, `variants[]`
- proves: the user-facing scope is Variant; each row carries the ConfigHub
  space, unit, object, risk, and next action from the shared workflow file.

### `node cli.mjs findings --json`

- mutates: no
- output shape: JSON object
- stable fields: `verdict` (`PASS`|`WATCH`), `reason` (a live-binding gate code
  while a high-severity finding is open, `NO_BLOCKING_FINDINGS` otherwise),
  `findings[]` rows with `severity`, `code`, `message`, `nextAction`
- proves: proof gaps, blockers, and stop rules are surfaced before preview,
  approval, or commit.

### `node cli.mjs preview --variant <id> --json`

- mutates: no (`"mutation": "none"` in the output)
- output shape: JSON object
- stable fields: `status` = `PREVIEW_READY`, `mutation` = `none`, `variant`,
  `approvalScope`
- proves: the scoped change can be described without mutating ConfigHub, Git,
  a controller, or runtime.

### `node cli.mjs commit --variant <id> --json`

- mutates: no in this starter — commit is gated until live bindings and a
  governed action executor exist
- output shape: JSON object at exit 0 (a typed refusal, not a shell failure)
- stable fields: `verdict` = `BLOCK`, `reason` =
  `APPROVED_CONFIGHUB_MUTATION_REQUIRED` (no complete live bindings) or
  `LIVE_ACTION_EXECUTOR_REQUIRED` (bindings present, no scenario executor)
- proves: `commit` means an approved scoped ConfigHub mutation; the CLI
  accepting a command is not proof that a live operation happened.

### `node cli.mjs verify --json`

- mutates: no
- output shape: JSON object
- stable fields: `verdict` = `WATCH`, `reason` (`RUNTIME_PROOF_PENDING` once
  bindings are ready, otherwise the live-binding gate code), `liveBindings`,
  `proofTabs[]`
- proves: the workflow is not called complete until ConfigHub and runtime
  proof agree.

### `node cli.mjs receipt --json`

- mutates: no
- output shape: JSON object
- stable fields: `verdict` = `WATCH`, `reason` (`SCENARIO_EXECUTOR_NOT_RUN`
  once bindings are ready, otherwise the live-binding gate code), `status` =
  `WAITING_FOR_LIVE_PROOF`, `proofTabs[]`, `omissions[]`
- proves: the receipt states what did not happen (omissions) instead of
  implying a live action ran.

### `npm run binding:check`

- mutates: no
- output shape: JSON object with `verdict` and `reason`; exit 0 for every
  expected state, non-zero only when `data/live-bindings.json` exists but
  cannot be read or parsed
- fresh clone: `{"verdict": "WATCH", "reason": "LIVE_BINDINGS_MISSING"}`
- proves: the live-binding contract (ConfigHub object, approval object,
  governed action contract, action endpoint, proof receipt, runtime evidence)
  is validated before commit can ever be considered.

### Binding gates

The three gates automation should branch on, in the order they appear:

| Gate code | Where | Meaning |
|---|---|---|
| `LIVE_BINDINGS_MISSING` | `preflight`, `findings`, `verify`, `receipt`, `binding:check` | No deployment-local `data/live-bindings.json` yet — the correct safe default on a fresh clone. |
| `APPROVED_CONFIGHUB_MUTATION_REQUIRED` | `commit` | Live bindings are absent or incomplete; commit refuses because commit means an approved scoped ConfigHub mutation. |
| `LIVE_ACTION_EXECUTOR_REQUIRED` | `commit` | Live bindings are present, but this generated starter ships no scenario-specific governed action executor. |

- mutates: no — every gate is a typed classification at exit 0
- proves: the app cannot drift from review into live mutation without real
  bindings, real approval, and a real executor.

### Proof-tab layers

The six proof layers every surface renders, from `proofTabs` in
`data/operational-workflow.json` (all start as `waiting` in this starter):

| Layer | Tab |
|---|---|
| `confighub` | ConfigHub revision |
| `approval` | Approval record |
| `policy` | Gate verdict |
| `controller` | Controller reconciliation |
| `runtime` | Runtime field |
| `receipt` | Receipt |

- mutates: no — the tabs are read-only proof state
- proves: completion is defined as ConfigHub, approval, policy, controller,
  runtime, and receipt evidence agreeing — not as a command exiting 0.
