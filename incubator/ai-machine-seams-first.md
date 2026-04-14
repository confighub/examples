# Machine Seams First

Use this guide when you want to evaluate an incubator example with an AI assistant without jumping straight into mutation.

This is the preferred family-wide pattern for cold starts, repeat runs, and "show me what this would do" walkthroughs.

## The Three Seams

For runnable examples, prefer these machine-readable seams in this order:

1. `./setup.sh --explain`
2. `./setup.sh --explain-json`
3. `./verify.sh --json`

What they mean:

- `./setup.sh --explain` = human-readable plan, read-only
- `./setup.sh --explain-json` = machine-readable plan, read-only
- `./verify.sh --json` = machine-readable proof after setup, read-only

If an example has these seams, an AI assistant should use them before reading shell internals or mutating ConfigHub.

## Default Evaluation Path

Use this order unless the human explicitly asks for a different mode:

1. orient with the example `README.md` and `AI_START_HERE.md`
2. run `./setup.sh --explain`
3. run `./setup.sh --explain-json | jq`
4. only run `./setup.sh` if the human wants a real operational proof
5. after setup, run `./verify.sh`
6. then run `./verify.sh --json | jq`
7. stop before cleanup unless the human asks

## What This Gives You

This pattern helps an AI answer the questions that matter early:

- what this example will read
- what this example will write
- which spaces and units are involved
- whether the example is still read-only
- what exact proof is available after a real run

It also makes it easier to compare examples consistently.

## Start Order For `global-app-layer`

If you want the shortest reliable learning path, use this order:

1. [`single-component`](./global-app-layer/single-component/README.md)
2. [`frontend-postgres`](./global-app-layer/frontend-postgres/README.md)
3. [`realistic-app`](./global-app-layer/realistic-app/README.md)
4. [`gpu-eks-h100-training`](./global-app-layer/gpu-eks-h100-training/README.md)

Reason:

- `single-component` is the smallest clean layered recipe
- `frontend-postgres` adds one more real dependency without too much surface area
- `realistic-app` is the best general-purpose app-shaped proof
- `gpu-eks-h100-training` is the most specialized and benefits from the shared seam pattern

## Recommended Commands

At repo root:

```bash
git rev-parse --show-toplevel
./scripts/verify.sh
```

Inside an example:

```bash
./setup.sh --help
./setup.sh --explain
./setup.sh --explain-json | jq
./verify.sh --help
./verify.sh --json | jq
```

For `global-app-layer`, read-only discovery also includes:

```bash
cd incubator/global-app-layer
./find-runs.sh --json | jq
```

## What Good AI Behavior Looks Like

- starts with the seams instead of mutating first
- distinguishes preview from operational proof
- says clearly what is and is not proven at each stage
- uses `--json` output for exact proof instead of ad hoc parsing
- only runs `./setup.sh` after the human asks for a real run

## What To Watch For

These are signs the AI is drifting:

- reading large shell files before trying `--explain-json`
- treating `README` prose as proof instead of running the seams
- mutating right after orientation without a clear ask
- using `grep`, `head`, or guessed output paths where `--json` already exists
- concluding an example "works" from preview output alone

## Scope Note

This pattern is strongest today for the `global-app-layer` examples. Some older incubator examples still rely more on human-readable scripts and may need additional cleanup before they are equally cold-start friendly.

## Related

- [AI_START_HERE.md](./AI_START_HERE.md)
- [ai-cold-eval-prompt-pack.md](./ai-cold-eval-prompt-pack.md)
- [ai-example-playbook.md](./ai-example-playbook.md)
- [ai-guide-standard.md](./ai-guide-standard.md)
- [global-app-layer/README.md](./global-app-layer/README.md)
