# Incubator AI Example Template

Use this as a starter for a major incubator example.

## Recommended File Bundle

```text
README.md
AI_START_HERE.md
prompts.md
contracts.md
```

## `README.md`

Suggested sections:

```md
# Example Name

## Stack And Scenario

## What This Proves

## Prerequisites

## What This Reads And Writes

## Read-Only Preview

## Run It

## Expected Output

## Verify It

## Inspect It In The GUI

## Troubleshooting

## Cleanup
```

## `AI_START_HERE.md`

Suggested sections:

```md
# AI Start Here

## What this example is for

## Safe first steps

## Capability branching

## Exact commands to run

## What mutates what

## What success looks like

## Cleanup
```

## `prompts.md`

Suggested prompts:

```md
## Orient Me First
Read this example, do not mutate anything yet, and explain:
- what stack it is for
- what it reads
- what it writes
- what I need installed
- what success should look like

## Safe Walkthrough
Guide me through this example step by step.
Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- tell me what success looks like
- tell me what to inspect next

## Verify Everything
After running the example, verify:
- ConfigHub objects
- GUI state
- target/worker readiness
- live apply or GitOps state if available
```

## `contracts.md`

Suggested structure:

```md
# Contracts

## Read-only contracts

### `cub space list --json`
- mutates: no
- stable fields: document them
- proves: document it

### `./setup.sh --explain-json`
- mutates: no
- stable fields: document them
- proves: document it
```

## Review Checklist

Before merging:

- does the README answer the six key reader questions?
- is there a read-only first path?
- is there a short AI guide?
- are there copyable prompts?
- are expected outputs documented?
- are cleanup steps documented?
- is there a stable JSON or text contract?
