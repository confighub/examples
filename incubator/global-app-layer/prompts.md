# Copyable Prompts

Use these prompts with Codex, Claude, Cursor, or another assistant while working in `incubator/global-app-layer`.

## 1. Orient Me First

Read this package and do not mutate anything yet.

Explain:
- what stack this package is for
- what it reads
- what it writes
- what I need installed
- which example I should start with
- what success should look like

Then run only read-only preview commands.
Also tell me whether `cub` auth is available and whether any targets are visible.
Do not guess unsupported `cub` subcommands; check `cub --help` first if needed.

## 2. Safe Walkthrough

Guide me through `global-app-layer` step by step.

Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- tell me what success looks like
- tell me what GUI page or CLI object to inspect next
- stop and branch clearly if auth or targets are missing
- use the documented JSON/jq contracts rather than inventing field paths

Start with `realistic-app` unless you think another example is a better fit.

## 3. Database-Only Path

I want to understand the layered recipe model without deploying to a cluster.

Use `global-app-layer` in ConfigHub-only mode:
- preview first
- create the recipe chain
- verify it
- show me the important spaces, units, and recipe manifest
- do not bind a target or apply anything live
- include GUI checkpoints as you go

## 4. NVIDIA-Shaped Walkthrough

Use `gpu-eks-h100-training` to explain how NVIDIA AICR-style layers map to ConfigHub units and variants.

Start read-only.
Then materialize the example in ConfigHub.
Then verify it.
Only offer the live path if a real target exists.

## 5. Verify Everything

After running a `global-app-layer` example, verify:
- the created spaces
- the created units
- the layered variant ancestry
- the recipe manifest
- target binding if used
- live apply state if used

Summarize what definitely happened, what did not happen, and what still depends on missing infrastructure.
