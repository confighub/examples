# AI Cold Eval Prompt Pack

Use these prompts when you want a repeatable AI evaluation of an incubator example without improvising the rules every time.

Start with [ai-machine-seams-first.md](./ai-machine-seams-first.md).

## Prompt 1: Read-Only Orientation

```text
Read this example's README.md and AI_START_HERE.md, then orient me without mutating anything.

Use the example's machine seams first:
- ./setup.sh --explain
- ./setup.sh --explain-json
- ./verify.sh --json if it is meaningful before setup

For each step, report:
1. what you are testing
2. the command you ran
3. the important output
4. what this proves
5. what this does not prove
6. what you would do next

Do not run ./setup.sh unless I explicitly ask.
```

## Prompt 2: Fast Preview

```text
Help me evaluate this incubator example quickly but safely.

Start with the read-only machine seams:
- ./setup.sh --explain
- ./setup.sh --explain-json
- ./verify.sh --json if available and useful

Then tell me:
- what this example will read
- what this example will write
- which spaces and units are involved
- whether a target, worker, or cluster is required
- the smallest real operational proof path if I choose to continue

Do not mutate anything yet.
```

## Prompt 3: Smallest Operational Proof

```text
Run the smallest honest operational proof for this example.

Rules:
- start with ./setup.sh --explain and ./setup.sh --explain-json
- say clearly when you are crossing from preview into mutation
- after setup, run the exact verification path, including ./verify.sh and ./verify.sh --json when available
- stop before cleanup unless I explicitly ask

For each phase, report:
1. what you are testing
2. the commands you ran
3. the important output
4. what this proves
5. what this does not prove
6. what you will do next
```

## Prompt 4: Cross-Example Comparison

```text
Compare these incubator examples as cold-start AI targets:
- single-component
- frontend-postgres
- realistic-app
- gpu-eks-h100-training

For each example:
- use the machine seams first
- stay read-only unless I explicitly ask for setup
- report whether the preview contracts are sufficient without human rescue

Score each example on:
- first-step clarity
- read-only safety
- machine-readable proof quality
- mutation clarity
- verification clarity

End with:
- best first example for a new AI assistant
- best first example for a human operator
- which example still needs the most AI contract cleanup
```

## Suggested Scorecard

Use this table shape when you want comparable results:

| Example | First step clear? | Stayed read-safe? | `--explain-json` sufficient? | `verify.sh --json` sufficient? | Human rescue needed? |
|---|---|---|---|---|---|
| `single-component` | yes/no | yes/no | yes/no | yes/no | none/light/heavy |
| `frontend-postgres` | yes/no | yes/no | yes/no | yes/no | none/light/heavy |
| `realistic-app` | yes/no | yes/no | yes/no | yes/no | none/light/heavy |
| `gpu-eks-h100-training` | yes/no | yes/no | yes/no | yes/no | none/light/heavy |

## Recommended Starting Order

For cold AI evaluation, start here:

1. `incubator/global-app-layer/single-component`
2. `incubator/global-app-layer/frontend-postgres`
3. `incubator/global-app-layer/realistic-app`
4. `incubator/global-app-layer/gpu-eks-h100-training`

## Related

- [ai-machine-seams-first.md](./ai-machine-seams-first.md)
- [AI_START_HERE.md](./AI_START_HERE.md)
- [global-app-layer/README.md](./global-app-layer/README.md)
