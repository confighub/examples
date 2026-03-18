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
Also tell me whether `cub` auth works and whether any targets are visible.
Do not guess unsupported `cub` subcommands or JSON paths; check the docs and `--help` first.

## Safe Walkthrough

Guide me through `realistic-app` step by step.

Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- say what success looks like
- say what to inspect next in both CLI and GUI
- stop and branch clearly if auth or targets are missing
- use the documented jq anchors for inspection commands
- after setup, surface the printed GUI URLs and `.logs/*.latest.log` files
- do not treat `cub target list` or `./set-target.sh` as proof that apply will work
- run `../preflight-live.sh <space/target>` before the live branch
- prefer `./apply-live.sh` over ad hoc manual apply steps

## Verify Everything

After running `realistic-app`, verify:
- created spaces
- created units
- layered variant ancestry
- recipe manifest
- target binding if used
- live apply state if used
- summarize what definitely happened, what did not happen, and what still depends on missing infrastructure

## Whole Lifecycle Walkthrough

Guide me through the full `realistic-app` lifecycle.

Start read-only, then continue through:
- ConfigHub materialization
- live target binding
- live apply
- shared upstream update
- a custom downstream deployment variant

Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- say what success looks like
- say what to inspect next in both CLI and GUI
- use `whole-journey.md` and the documented contracts rather than guessing
- run `../preflight-live.sh <space/target>` before binding or applying
- only call the live path ready if preflight reports `applyReady: true`
- only call live apply successful if backend, frontend, and postgres all reach `Ready` with `ApplyCompleted`
