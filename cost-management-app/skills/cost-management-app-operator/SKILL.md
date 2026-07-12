---
name: cost-management-app-operator
description: 'Operate the Cost Management App app through its generated CLI and browser surface. Use for "Turn fleet cost findings into governed recommendations, approvals, and receipts.", Variant review, proof gaps, preview, approval scope, and receipt checks. Read-only first; live mutation requires the app commit command to prove real ConfigHub bindings.'
phase: verify
allowed-tools: Bash(node cli.mjs preflight --json) Bash(node cli.mjs map --json) Bash(node cli.mjs findings --json) Bash(node cli.mjs preview *) Bash(node cli.mjs verify --json) Bash(node cli.mjs receipt --json)
---

# Cost Management App Operator

Use this skill when a user asks to operate this generated ConfigHub app.

## Route

1. Run `node cli.mjs preflight --json`.
2. Run `node cli.mjs map --json` to show Variant scope.
3. Run `node cli.mjs findings --json` to show blockers and proof gaps.
4. Use `node cli.mjs preview --variant <variant-id> --json` before any live operation.
5. Treat `node cli.mjs commit --variant <variant-id> --json` as a governed mutation gate. If it blocks, report the blocker; do not bypass it.
6. Close with `node cli.mjs verify --json` and `node cli.mjs receipt --json`.

## Boundaries

- The CLI and GUI share `data/operational-workflow.json`.
- Deployment-local bindings live in `data/live-bindings.json`.
- Do not treat a missing live binding as success.
- Do not claim runtime success without receipt and runtime evidence.
