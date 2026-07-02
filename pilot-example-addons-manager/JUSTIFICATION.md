# Justification: Add-on Manager

## Why This Should Be An Operational App

Install, upgrade, review, and prove platform add-ons through governed ConfigHub delivery paths.

This workflow deserves an app when the operator needs more than a one-off
screen or command. The useful product is the repeatable path from current state
to approved action to proof.

The package is justified only if a new user can get from the workflow idea to a
tested scenario, browser GUI, CLI sibling, and visible proof gaps in about 15
minutes. That speed target does not lower the proof bar; it forces the package
to expose the proof bar clearly.

## Why ConfigHub Is The Authority

ConfigHub stores the operational objects that make this safe to review and
repeat: Variants, spaces, Units, revisions, approvals, actions, events, proof
receipts, and URLs.

## Why The CLI Sibling Exists

- A dashboard can show state, but an operational app must show scope, approval, action, and proof.
- The CLI sibling makes the workflow repeatable, scriptable, and easy for an assistant or operator to test.
- The browser GUI gives the operator a richer review surface without becoming a separate authority.
- ConfigHub remains the governed source for state, approval, action history, proof, and URLs.
- Variant-first language keeps the user on the app operating model while spaces, Units, filters, and targets remain implementation details.

## Commit Semantics

`commit` is intentionally strict. It means an approved, scoped ConfigHub
mutation using the governed action path. Until the live ConfigHub object,
approval object, governed action executor, proof receipt, and runtime evidence
are bound, the generated CLI must block commit instead of pretending the action
ran.

## Guardrail Semantics

Some workflows have policy guardrails. Others close through runtime proof,
approval proof, controller proof, receipt proof, or another proof gate. The app
may expose a `guardrails` command or screen when the scenario has that concept,
but guardrails are not a universal required phase.
