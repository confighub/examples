# AI Start Here

Use this page when you want to drive `connect-and-compare` safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. say whether there is a GUI checkpoint; for this example there is none
5. ask `Ready to continue?`
6. wait for the human before continuing

## Suggested Prompt

```text
Read connect-and-compare/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Do not continue until I say continue.
```

## What This Example Is For

This is a fixture-first evidence example.

Use it when the human wants to show:

- immediate standalone signal
- compare output
- history output
- no live cluster requirement

## Stage 1: Preview The Plan (read-only)

```bash
cd connect-and-compare
./setup.sh --explain
./setup.sh --explain-json | jq
```

What this does not mutate:

- ConfigHub
- live infrastructure
- any shared local state beyond reading fixtures

GUI checkpoint:

- none for this local-only example

Pause after this stage.

## Stage 2: Generate The Sample Evidence (local write only)

```bash
./setup.sh
```

What this writes:

- local sample output only

What you should see after:

- `sample-output/01-doctor.txt`
- `sample-output/03-compare.json`
- `sample-output/04-history.txt`

GUI checkpoint:

- none for this local-only example

Pause after this stage.

## Stage 3: Verify The Evidence (read-only against local output)

```bash
./verify.sh
jq '.alignment' sample-output/03-compare.json
jq '.alignment[] | select(.status != "aligned")' sample-output/03-compare.json
cat sample-output/01-doctor.txt
cat sample-output/04-history.txt
```

What this proves:

- doctor output proves immediate standalone signal
- compare output proves intent versus observed alignment facts
- history output proves the review trail story

What this does not prove:

- no ConfigHub mutation
- no live cluster state

GUI checkpoint:

- none for this local-only example

Pause after this stage.

## Stage 4: Cleanup (local only)

```bash
./cleanup.sh
```

This removes only local sample output.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
