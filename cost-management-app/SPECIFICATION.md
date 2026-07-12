# Specification: Cost Management App

## Purpose

Turn fleet cost findings into governed recommendations, approvals, and receipts.

This app is allowed to help an operator review affected Variants, preview a
proposed change, approve the exact scope, run only the connected governed
action, and verify the receipt.

## Original Request

```text
cost-management-opencost
```

## User-Facing Scope

The app speaks in Variants first.

- User-facing scope: Variant
- Implementation bindings: ConfigHub Space, ConfigHub Unit, Filter, Target, where predicate
- Rule: Show Variants to users first; compile to ConfigHub spaces, units, filters, targets, and predicates underneath.

## Shared Workflow Contract

All surfaces read the same files:

- `data/operational-workflow.json`: scenario, Variant scope, controls, approval
  fields, proof tabs, stop rules, and the CLI loop.
- `data/live-bindings.json`: deployment-local live ConfigHub object,
  organization identity, action authority, proof, and runtime bindings.

The browser GUI, CLI, local server, tests, and assistant skill stubs must not
invent their own scope, findings, approval path, or proof state.

## 15-Minute Acceptance Target

A new user should be able to verify the generated scenario, app, browser GUI,
and CLI sibling in about 15 minutes by running `npm run verify`, checking
`node cli.mjs preflight --json`, opening the GUI, and confirming that both
surfaces show the same Variant scope, findings, approval gate, action state,
proof gaps, and receipt path.

## CLI Loop

```text
preflight -> map/list -> findings -> preview -> review/commit -> verify -> receipt
```

| CLI command | Harness step | Product step | Purpose |
|---|---|---|---|
| `preflight` | Doorway / Gate | readiness check | Check package, auth mode, workflow contract, Variant inventory, and live-binding readiness before risk. |
| `map/list` | Intake | Map | Show affected Variants and their ConfigHub spaces, Units, URLs, risk, and next action. |
| `findings` | Route | Recommend | Show proof gaps, blockers, and the next safe action. |
| `preview` | Gate | Preview | Preview the scoped change without mutating ConfigHub, Git, a controller, or runtime. |
| `review/commit` | Review / Run gate | Operate | Local review records one exact, expiring packet but grants no permission; a separate explicit confirmation requests the ConfigHub write, whose revision and mutation parity are then verified. |
| `verify` | Prove | Verify | Check ConfigHub, approval, action, controller/runtime evidence, URL proof, and omissions. |
| `receipt` | Prove / Learn | Receipt | Leave a compact result that a human or automation can review and rerun. |

## Exact Review Rule

The review path does not approve a hand-entered function call. An actionable
finder row owns the target, function, and arguments. `preview --finding <id>`
reads the target Unit, checks its organization, runs the function in dry-run
mode against the exact head revision, and stores the resulting diff and digest.
`review --record --preview <id>` derives the reviewer from the authenticated Cub
context and refuses a changed revision. `commit --review <id> --confirm-execute`
re-authenticates the same user, checks the head again, executes only the
finding-owned function, and verifies an exact one-revision advance plus mutation
parity with the reviewed dry run. A successful receipt proves the ConfigHub
revision. It does not claim provider-native atomic enforcement or controller
delivery; those remain explicit proof layers.

The local review file is inspectable evidence, not a signed
ConfigHub approval or an authorization token. It cannot widen the finding's
scope and cannot unlock mutation without the separate explicit execution
confirmation.

Blocked action authority, a stale or tampered preview, an org mismatch, or a
changed Unit is a typed non-PASS result. Missing provider-native atomicity is
an explicit receipt caveat, while a mismatched actual diff or unexpected
revision sequence makes the execution receipt non-PASS. Controller and runtime evidence are a
separate delivery gate; a reviewed ConfigHub change is not a live rollout.

## App Lifecycle Commands

The workflow loop is the app's job. The app's own life is covered by
`lifecycle.mjs`, required in every export:

- install
- upgrade
- migrate
- rollback
- rotate-auth
- decommission

`upgrade` diffs a regeneration (generator version pinned in
`app-export-manifest.json`) against local modifications. `migrate` applies
schema-version migrations for the shared contract files. `rotate-auth` records
the client rotation in the fleet record's `oauthClient` block and refuses to
skip registration states (`unregistered -> registered -> rotated -> revoked`).
`decommission` marks the client registration `revoked` and
deregisters the app from the org-level fleet index.

## Result Contract

Every `cli.mjs`, `lifecycle.mjs`, and `binding:check` result is JSON carrying a
`verdict` and a stable `reason`:

| `verdict` | Meaning | Exit |
|---|---|---|
| `PASS` | The action completed. | 0 |
| `WATCH` | Not proven yet / waiting (e.g. `LIVE_BINDINGS_MISSING`, upgrade conflicts). | 0 |
| `BLOCK` | Refused: the action cannot proceed as asked (e.g. `commit` with no live bindings, an invalid registry transition). | 0 |
| `ASK` | A human decision is required (e.g. `decommission` without `--confirm`). | 0 |
| `ERROR` | The command could not run: malformed input, a missing core file it must parse, a parse failure, or an unknown command. | non-zero |

An expected blocker is a successful, scriptable classification: `PASS`, `WATCH`,
`BLOCK`, and `ASK` all **exit 0**. A non-zero exit means `ERROR` only. Branch on
`verdict` and `reason`, never on shell success. The single runtime exception is
`node server.mjs` after `decommission`, which refuses to start and exits 3.

## Approval Scope

- org
- space
- variant
- unit
- action
- revision
- target
- strategy
- cost finding
- field

`commit` means an explicit execution request for one short-lived stored review,
recorded by the authenticated ConfigHub user and still bound to the same Unit
identity and revision. The executor can write the finding-owned function, but
it must then prove one ConfigHub revision and exact mutation parity. Provider
atomicity remains `WATCH`; the CLI accepting a command is not proof that a
mutation, controller delivery, or live rollout happened.

## Stop Rules

- Do not run a live action without a signed-in ConfigHub user.
- Do not run a live action without exact Variant scope.
- Do not run a live action without approval.
- Do not call the workflow complete until ConfigHub and runtime proof agree.

## Proof Tabs

- ConfigHub revision (confighub)
- Approval record (approval)
- Gate verdict (policy)
- Receipt (receipt)
