---
name: cost-management-app-operator
description: 'Operate the Cost Management App app through its generated CLI and browser surface. Use for "Turn fleet cost findings into governed recommendations, approvals, and receipts.", Variant review, proof gaps, preview, approval scope, and receipt checks. Read-only first; live mutation requires the app commit command to prove real ConfigHub bindings.'
phase: verify
allowed-tools: Bash(node cli.mjs preflight --json) Bash(node cli.mjs map --json) Bash(node cli.mjs findings --json) Bash(node cli.mjs preview *) Bash(node cli.mjs review *) Bash(node cli.mjs commit *) Bash(node cli.mjs verify --json) Bash(node cli.mjs receipt --json)
---

# Cost Management App Operator

Use this skill when a user asks to operate this generated ConfigHub app.

## Route

1. Run `node cli.mjs preflight --json`.
2. Run `node cli.mjs map --json` to show Variant scope.
3. Run `node cli.mjs findings --json` to show blockers and proof gaps.
4. Choose an actionable id from `node cli.mjs findings --json`, then run `node cli.mjs preview --finding <finding-id> --json` and inspect the saved diff.
5. Record the exact review with `node cli.mjs review --record --preview <preview-id> --reason '<why>' --json`; never hand-author a function, target, or argument at review time. Show the human the target, function, arguments, and reviewed diff. The record is evidence, not approval.
6. Ask one explicit approval question for that exact review id and scope. Only after the human approves, run `node cli.mjs commit --review <review-id> --confirm-execute --json`. If it blocks or returns an unverified receipt, report that state; do not retry automatically.
7. Close with `node cli.mjs verify --json` and `node cli.mjs receipt --json`.

## Boundaries

- The CLI and GUI share `data/operational-workflow.json`.
- Deployment-local bindings live in `data/live-bindings.json`.
- Review files are local evidence, not signed ConfigHub approvals or mutation permission.
- Do not treat a missing live binding as success.
- Do not claim runtime success without receipt and runtime evidence.
