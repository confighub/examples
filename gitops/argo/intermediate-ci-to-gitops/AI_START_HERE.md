# AI Start Here

Use this page when you want to drive
`gitops/argo/intermediate-ci-to-gitops` safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. say what the GUI shows today
6. say what the GUI does not show yet
7. name the GUI feature ask and cite the issue number if one exists; if not, say that explicitly
8. tell the human to open the GUI and give them time to inspect it
9. ask `Ready to continue?`
10. do not move on until the human says to continue

## Suggested Prompt

```text
Read gitops/argo/intermediate-ci-to-gitops/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This is an intermediate-level Argo CD layout: an application repo whose CI
opens a pull request against a separate GitOps repo, and a promotion to
production that is a second pull request copying a tag. Both workflow files
are illustrative only; this example never runs them, never builds an image,
and never opens a pull request anywhere. It does not create a cluster and
does not install Argo CD.

## Stage 1: Preview The Plan (read-only)

```bash
cd gitops/argo/intermediate-ci-to-gitops
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not touch any cluster, build
any image, or open any pull request.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI surface for the render plan before upload
- GUI feature ask: no issue filed yet for a plan-oriented GUI handoff on this example

Pause after this stage.

## Stage 2: Render And Upload (mutates ConfigHub)

```bash
./setup.sh
```

What this mutates:

- creates or updates ConfigHub Space for the dev render
- creates or updates ConfigHub Space for the prod render

What this does not mutate:

- no live cluster, no Argo CD installation, no container build, no pull
  request in either repo

GUI checkpoint:

- GUI now: open the two Spaces in ConfigHub and inspect the uploaded Units
- GUI gap: there is no single view that shows both environments, or the two
  pull requests that produced them, side by side
- GUI feature ask: no issue filed yet for a dev/prod side-by-side view

Pause after this stage.

## Stage 3: Verify The Evidence (read-only)

```bash
./verify.sh
```

This renders both overlays locally, checks the expected resources are
present, checks that dev and prod differ, and checks that the prod image tag
matches what `PROMOTED_FROM.md` says was promoted. It does not call
ConfigHub, so it will pass even if you skipped Stage 2.

GUI checkpoint:

- GUI now: none for this stage; verification is local
- GUI gap: none identified
- GUI feature ask: none

Pause after this stage.

## Stage 4: Break It On Purpose (optional, local edits only)

The README's "Break it on purpose" section has one single-edit breakage: a
promotion pull request merged with the wrong tag. Make the edit, run
`./verify.sh`, read the failure, then read "What ConfigHub shows that the PR
diff does not" in the README. Undo the edit with
`git checkout -- environments/apptique/prod/kustomization.yaml` (from inside
`gitops-repo/`) before moving on.

GUI checkpoint:

- GUI now: the prod Space's Unit diff, if you ran setup.sh again with the bad
  tag in place
- GUI gap: nothing surfaces the promotion record (`PROMOTED_FROM.md`) next to
  the Unit diff; a reader has to know to compare them
- GUI feature ask: no issue filed yet for surfacing a promotion record next
  to the Unit diff it should match

Pause after this stage.

## Stage 5: Cleanup

```bash
./cleanup.sh
```

This removes local rendered output under `var/`. It prints, but does not
run, the `cub space delete` commands for the Spaces Stage 2 created.

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
