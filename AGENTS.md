# AGENTS.md

This repo is designed to be usable by both humans and AI assistants.

If you are an AI assistant, treat this file as the short, strict cold-start protocol.
Use [AI-README-FIRST.md](./AI-README-FIRST.md) only after you have followed this file.

## 1. Resolve the repo root first

Do not hardcode the checkout path.

This repo may be checked out as `examples`, `confighub-examples`, or another directory name.

Run:

```bash
git rev-parse --show-toplevel
```

## 2. Use the current CLI and code as source of truth

`docs.confighub.com` may be out of date.

For current behavior, prefer:

- local `cub` help output
- `sdk/cmd/cub` source in the main ConfigHub codebase
- current example scripts in this repo

Do not invent commands because they sound plausible.

## 3. Access live ConfigHub through `cub`, not the web app

If a user asks whether you can access ConfigHub, do not start with `https://hub.confighub.com`.

Use:

```bash
cub context list --json
cub space list --json
cub target list --space "*" --json
```

These are read-only.

## 4. Default to read-only first

Use this cold-start sequence:

```bash
git rev-parse --show-toplevel
./scripts/verify.sh
cub context list --json
cub space list --json
cub target list --space "*" --json
```

These commands do not mutate:

- spaces
- units
- targets
- workers
- clusters

## 5. For `global-app-layer`, do this before reading shell code

```bash
cd incubator/global-app-layer
./find-runs.sh --json | jq
cd realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

This is the shortest safe path to understanding:

- what live runs already exist
- what the example will create
- what spaces, units, and layers are involved
- what commands the setup flow uses

These commands are read-only.

## 6. Use `--where` for label filtering

Do not guess `--label` on list commands.

Correct:

```bash
cub space list --where "Labels.ExampleName = 'global-app-layer-realistic-app'" --json
cub unit list --space "*" --where "Labels.Component = 'backend'" --json
```

Wrong:

```bash
cub space list --label "ExampleName=..."
```

## 7. Distinguish read-only from mutating actions

Read-only:

- `./scripts/verify.sh`
- `cub context list --json`
- `cub space list --json`
- `cub target list --space "*" --json`
- `./find-runs.sh`
- `./setup.sh --explain`
- `./setup.sh --explain-json`
- `cub unit get --json`
- `cub function do --dry-run --json ...`
- `cub unit apply --dry-run --json ...`

Mutating:

- `./setup.sh`
- `./set-target.sh`
- `cub unit apply`
- `cub function do`
- `cub unit create`
- `cub space create`

Before mutating, say what will change.

## 7a. Evaluation modes

When a user asks to "evaluate" an example, do not guess what level of proof they want.

Use these shared meanings:

- **preview** = read-only orientation only
- **fast preview** = the example's read-only path (`--explain`, `--explain-json`, read-only demo/report scripts)
- **operational evaluation** = run the smallest real setup/proof path that shows the example actually works
- **guided walkthrough** = pause-heavy presenter mode following the stage structure in `AI_START_HERE.md`

If the user says "evaluate it quickly" or "use the fast path" and does **not** explicitly say "read-only":

1. start with the fast preview
2. then continue into the smallest real operational proof if the example has:
   - `./setup.sh`
   - `./verify.sh`
   - a representative proof action documented in `contracts.md` or `AI_START_HERE.md`
3. stop before cleanup unless the user asks for cleanup

Only stay fully read-only when the user explicitly asks for:

- preview only
- read-only only
- explanation only

The goal is to avoid the failure mode where the AI reviews the docs, runs only preview commands, and then claims the example is "ready" without proving that setup and one representative action actually work.

## 8. Prefer exact, machine-readable output

For AI work, prefer:

- `--json`
- `--jq`
- `./setup.sh --explain-json`
- `./find-runs.sh --json`

Do not parse human table output if JSON is available.

## 9. Most useful files

- human entry path: [START_HERE.md](./START_HERE.md)
- AI context: [AI-README-FIRST.md](./AI-README-FIRST.md)
- incubator AI path: [incubator/AI_START_HERE.md](./incubator/AI_START_HERE.md)
- layered examples: [incubator/global-app-layer/README.md](./incubator/global-app-layer/README.md)
- layered run discovery: [incubator/global-app-layer/find-runs.sh](./incubator/global-app-layer/find-runs.sh)

## 10. If unsure

Stop guessing and inspect one of:

- `cub <command> --help`
- `sdk/cmd/cub/*.go`
- the example README
- `./setup.sh --explain-json`
