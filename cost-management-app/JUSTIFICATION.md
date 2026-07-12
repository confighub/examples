# Justification: Cost Management App

## Why This Should Be An Operational App

Turn fleet cost findings into governed recommendations, approvals, and receipts.

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

`commit` is intentionally strict. A finding owns the action, a dry-run preview
owns the exact diff and revision, and the authenticated ConfigHub user records
a short-lived review. That record is evidence, not permission. A separate
explicit confirmation requests the write. The generated CLI rechecks identity,
freshness, and head revision, runs only the finding-owned function, and proves
the resulting revision and mutation parity. Provider-native atomic
expected-revision enforcement remains `WATCH` until ConfigHub exposes it.
Controller and runtime proof remain later gates; a committed ConfigHub revision
must not be called delivered or live without that evidence.

## Guardrail Semantics

Some workflows have policy guardrails. Others close through runtime proof,
approval proof, controller proof, receipt proof, or another proof gate. The app
may expose a `guardrails` command or screen when the scenario has that concept,
but guardrails are not a universal required phase.
