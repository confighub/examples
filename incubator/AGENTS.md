# Incubator AI Protocol

Use this file when you are working anywhere under `examples/incubator/`.

This is the shortest safe AI starting point for experimental examples.

## What This Area Is

`incubator/` holds example work that is still being shaped:
- new walkthroughs
- new delivery patterns
- layered recipe experiments
- proof-of-concept UX for humans and AI

These examples are more experimental than the stable examples at the repo root.

## First Rule

Start read-only.

Before mutating ConfigHub or a live cluster, do three things:

1. explain what the example is for
2. show the read-only preview path
3. say what the next command will read and write

## Start Here

Read these in order:

1. [`README.md`](./README.md)
2. [`AI_START_HERE.md`](./AI_START_HERE.md)
3. [`AI-README-FIRST.md`](./AI-README-FIRST.md)

If you are reviewing or authoring an incubator example, also read:

4. [`ai-example-playbook.md`](./ai-example-playbook.md)
5. [`ai-example-template.md`](./ai-example-template.md)

## Non-Negotiable Questions

Every important incubator example should answer:

1. what stack is this for?
2. what do I need installed?
3. what does this read?
4. what does this write?
5. what should I expect to see?
6. how would an AI assistant run this safely?

If the docs do not answer these near the top, the AI should say that plainly and proceed cautiously.

## Major Example Bundle

Every major incubator example should ideally include:

1. a short human `README.md`
2. a short AI guide
3. copyable prompts
4. expected output or expected state
5. cleanup steps
6. stable JSON or text contracts

## Read-Only Defaults

Prefer these kinds of commands first:

- `./scripts/verify.sh`
- `cub version`
- `cub context list`
- `cub ... --help`
- `cub ... --json`
- example-specific `--explain`
- example-specific `--explain-json`

When available, prefer the example's own durable artifacts over scrollback:

- printed GUI URLs from `setup.sh`
- `.logs/setup.latest.log`
- `.logs/verify.latest.log`
- `.logs/cleanup.latest.log`

For `global-app-layer`, the safest first commands are:

```bash
cd incubator/global-app-layer
./find-runs.sh --json | jq
cd realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Walkthrough Pattern

When guiding a human, always say:

1. what this step does
2. whether it mutates anything
3. what success looks like
4. what to inspect next in the GUI or CLI

## If Live Infra Is Missing

Do not get stuck trying to force the live path.

If there is:
- no auth
- no worker
- no target
- no cluster

then continue with the highest-fidelity database-only or explain-only path and say clearly what was not exercised.

## CLI Footguns To Avoid

- use `cub version`, not `cub --version`
- use `cub context list`, not `cub context current`
- check `cub ... --help` before assuming a subcommand or JSON shape
