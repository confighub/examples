# Specification: Add-on Manager

## Purpose

Install, upgrade, review, and prove platform add-ons through governed ConfigHub delivery paths.

This app is allowed to help an operator review affected Variants, preview a
proposed change, approve the exact scope, run only the connected governed
action, and verify the receipt.

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
  approval, action, proof, and runtime bindings.

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
preflight -> map/list -> findings -> preview -> approve/commit -> verify -> receipt
```

| CLI command | Harness step | Product step | Purpose |
|---|---|---|---|
| `preflight` | Doorway / Gate | readiness check | Check package, auth mode, workflow contract, Variant inventory, and live-binding readiness before risk. |
| `map/list` | Intake | Map | Show affected Variants and their ConfigHub spaces, Units, URLs, risk, and next action. |
| `findings` | Route | Recommend | Show proof gaps, blockers, and the next safe action. |
| `preview` | Gate | Preview | Preview the scoped change without mutating ConfigHub, Git, a controller, or runtime. |
| `approve/commit` | Run after approval | Operate | Commit only means an approved scoped ConfigHub mutation through the governed action path. |
| `verify` | Prove | Verify | Check ConfigHub, approval, action, controller/runtime evidence, URL proof, and omissions. |
| `receipt` | Prove / Learn | Receipt | Leave a compact result that a human or automation can review and rerun. |

## Approval Scope

- org
- space
- variant
- unit
- action
- revision
- target
- strategy
- addon
- version

`commit` means an approved scoped ConfigHub mutation. The CLI accepting a
command is not proof that a live operation happened.

## Stop Rules

- Do not run a live action without a signed-in ConfigHub user.
- Do not run a live action without exact Variant scope.
- Do not run a live action without approval.
- Do not call the workflow complete until ConfigHub and runtime proof agree.

## Proof Tabs

- ConfigHub revision (confighub)
- Approval record (approval)
- Gate verdict (policy)
- Controller reconciliation (controller)
- Runtime field (runtime)
- Receipt (receipt)
