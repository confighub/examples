# Copyable Prompts

## Orient Me First

Read `single-component` and do not mutate anything yet.

Explain:
- what stack it is for
- what it reads
- what it writes
- what I need installed
- what success should look like

Then run only the read-only preview commands.
Also tell me whether `cub` auth works and whether any targets are visible.
Do not guess unsupported `cub` subcommands or JSON paths; check the docs and `--help` first.

## Safe Walkthrough

Guide me through `single-component` step by step.

Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- say what success looks like
- say what to inspect next in both CLI and GUI
- stop and branch clearly if auth or targets are missing
- use the documented jq anchors for inspection commands
- after setup, surface the printed GUI URLs and `.logs/*.latest.log` files

## Verify Everything

After running `single-component`, verify:
- created spaces
- created units
- layered variant ancestry
- recipe manifest
- target binding if used
- live apply state if used
- summarize what definitely happened, what did not happen, and what still depends on missing infrastructure
