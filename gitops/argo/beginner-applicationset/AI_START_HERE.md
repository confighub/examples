# AI Start Here

Use this page when you want to drive `gitops/argo/beginner-applicationset`
safely with an AI assistant.

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
Read gitops/argo/beginner-applicationset/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This is a beginner-level Argo CD ApplicationSet layout: one app, two
environments (dev, prod), discovered by a directory generator. It renders
with Kustomize and uploads into ConfigHub. It does not create a cluster and
does not install Argo CD.

## Stage 1: Preview The Plan (read-only)

```bash
cd gitops/argo/beginner-applicationset
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not touch any cluster.

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

- creates or updates ConfigHub Space `gitops-argo-beginner-dev`
- creates or updates ConfigHub Space `gitops-argo-beginner-prod`

What this does not mutate:

- no live cluster, no Argo CD installation

GUI checkpoint:

- GUI now: open the two Spaces in ConfigHub and inspect the uploaded Units
- GUI gap: there is no single view that shows both environments side by side for this example
- GUI feature ask: no issue filed yet for a dev/prod side-by-side view

Pause after this stage.

## Stage 3: Verify The Evidence (read-only)

```bash
./verify.sh
```

This re-renders both overlays locally and checks the expected resources are
present. It does not call ConfigHub, so it will pass even if you skipped
Stage 2.

GUI checkpoint:

- GUI now: none for this stage; verification is local
- GUI gap: none identified
- GUI feature ask: none

Pause after this stage.

## Stage 4: Cleanup

```bash
./cleanup.sh
```

This removes local rendered output under `var/`. It prints, but does not
run, the `cub space delete` commands for the Spaces Stage 2 created.

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
