# AI Start Here

Use this page when you want to walk through
`gitops/argo/intermediate-git-as-database` with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. open the file in `repo/` that shows the problem, and quote the lines
5. say what the GUI shows today
6. say what the GUI does not show yet
7. name the GUI feature ask and cite the issue number if one exists; if not, say that explicitly
8. ask `Ready to continue?`
9. do not move on until the human says to continue

## Rules for this example

- Every number you state must come from scout's output or a script you ran.
  Never estimate one.
- If something cannot be determined from `repo/`, say NOT OBSERVED.
- Treat each finding as a question for the repo's owner, not a defect.
- Do not mention ConfigHub in the answers to the seven questions. This stage of
  the example is the "before".

## Suggested Prompt

```text
Read gitops/argo/intermediate-git-as-database/AI_START_HERE.md and walk me through it.
Pause after every stage. Show full output.
For each of the seven questions, open the file in repo/ that shows it and quote the lines.
Do not continue until I say continue.
```

## What This Example Is For

A small Argo CD fleet repo (24 clusters, two ApplicationSets, one chart) with
one planted instance of each of seven problems that appear when a Git
repository becomes the operational store for a fleet. The answers are in
`EXPECTED.md`. Nothing here mutates ConfigHub, a cluster, or any file.

## Stage 1: Preview The Plan (read-only)

```bash
cd gitops/argo/intermediate-git-as-database
./setup.sh --explain
./setup.sh --explain-json | jq
```

GUI checkpoint:

- GUI now: none; this example is CLI-only
- GUI gap: the example is not uploaded into ConfigHub yet, so there is nothing to open
- GUI feature ask: no issue filed yet for a ConfigHub "after" of this fleet

Pause after this stage.

## Stage 2: Run The Seven Questions (read-only)

```bash
pip install pyyaml
./setup.sh
```

Walk the output one question at a time. For each, open the file named in
`README.md` under "The seven, one at a time" and show the lines.

GUI checkpoint:

- GUI now: none
- GUI gap: reach per change, promotion state and drift tolerance are not shown anywhere for this fleet
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 3: Check Against The Answers (read-only)

```bash
./verify.sh
```

Success text: `All checks passed.` Compare scout's numbers with `EXPECTED.md`
and say which of the seven scout answers fully and which need a person (see
"Not planted" in `EXPECTED.md`).

GUI checkpoint:

- GUI now: none
- GUI gap: none for this stage
- GUI feature ask: none

Pause after this stage.

## Stage 4: Optional, Test An AI Tool

Paste `prompts/config-repo-ten-questions.md` into a fresh agent session opened
on `repo/`, then compare its answers with `EXPECTED.md`. Flag any number the
agent stated without computing it.
