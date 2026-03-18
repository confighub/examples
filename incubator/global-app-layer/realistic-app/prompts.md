# Copyable Prompts

## Orient Me First

Read `realistic-app` and do not mutate anything yet.

Explain:
- what stack it is for
- what it reads
- what it writes
- what I need installed
- what success should look like

Then run only the read-only preview commands.

## Safe Walkthrough

Guide me through `realistic-app` step by step.

Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- say what success looks like
- say what to inspect next

## Verify Everything

After running `realistic-app`, verify:
- created spaces
- created units
- layered variant ancestry
- recipe manifest
- target binding if used
- live apply state if used
