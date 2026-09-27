# AI Start Here

Use this page when you want to drive `gitops/expert-broken-states` safely
with an AI assistant.

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
Read gitops/expert-broken-states/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This is one small app (`apptique`), delivered by an Argo CD Application and
a Flux Kustomization, in three states: healthy, drift, failed sync, and bad
commit. Everything renders offline. This example never installs Argo CD or
Flux, never applies anything, and never touches a cluster. Only the
healthy state is ever uploaded to ConfigHub.

## Facts To Get Right Before You Say Anything

- One app: `apptique`, one namespace: `apptique-broken-states`.
- Two control objects deliver it: an Argo CD `Application`
  (`argo/application.yaml`) and a Flux `Kustomization` plus `GitRepository`
  (`flux/apps.yaml`, `flux/gitrepository.yaml`), both pointed at
  `apps/apptique/overlays/healthy`.
- `overlays/failed-sync` and `overlays/bad-commit` both render and both
  differ from `overlays/healthy` by exactly one thing: an added
  `RedisCache` custom resource, or a Service `targetPort` moved to 8080
  while the container stays on 80. Both build on `overlays/healthy` and
  render into the same `apptique-broken-states` namespace, so the
  scenarios' diagnostics read that namespace. Neither is ever uploaded to ConfigHub or
  applied to a cluster by this example's scripts. The bad-commit scenario
  page documents an optional upload a person may run in their own
  terminal; do not run it for them without asking, because it mutates
  ConfigHub.
- Drift has no overlay: it is a live divergence, not a manifest state, so
  it is documented rather than rendered.

## Stage 1: Preview The Plan (read-only)

```bash
cd gitops/expert-broken-states
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not touch any cluster.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI surface for the render plan before upload
- GUI feature ask: no issue filed yet for a plan-oriented GUI handoff on this example

Pause after this stage.

## Stage 2: Render The Healthy State And Both Control Objects (read-only)

```bash
kustomize build apps/apptique/overlays/healthy
kustomize build argo
kustomize build flux
```

Say out loud, before moving on: which namespace the app lands in, and which
two objects (the Argo `Application`, the Flux `Kustomization`) would each
deliver it there.

GUI checkpoint:

- GUI now: none; this stage reads the repo only
- GUI gap: no side-by-side of the Argo and Flux control objects for the same app
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 3: Render The Two Broken States (read-only)

```bash
kustomize build apps/apptique/overlays/failed-sync
kustomize build apps/apptique/overlays/bad-commit
```

Both commands succeed. Say out loud, before moving on: for each one, is the
problem in this rendered YAML, or only visible once something tries to
apply or run it? (Neither: the YAML is well-formed both times. The
failed-sync problem is a missing CRD on the API server; the bad-commit
problem is a runtime mismatch: the Service sends traffic to port 8080,
and the container listens on 80.)

GUI checkpoint:

- GUI now: none; local render only
- GUI gap: no visual diff between the healthy overlay and either broken one
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 4: Render And Upload The Healthy State Only (mutates ConfigHub)

```bash
./setup.sh
```

What this mutates:

- creates or updates ConfigHub Space
  `gitops-expert-broken-states-argo-control` (the Argo Application)
- creates or updates ConfigHub Space
  `gitops-expert-broken-states-flux-control` (the Flux Kustomization and
  GitRepository)
- creates or updates ConfigHub Space `gitops-expert-broken-states` (the
  healthy `apptique` Unit)

A ConfigHub Space belongs to one Component, so the Argo and Flux control
objects each get their own Space rather than sharing one: uploading a
second Component into an already-linked Space would re-link it and
overwrite its labels.

What this does not mutate:

- no live cluster, no Argo CD or Flux installation, no Git history
- the `failed-sync` and `bad-commit` overlays are never uploaded

GUI checkpoint:

- GUI now: open the three Spaces in ConfigHub and inspect the uploaded Units
- GUI gap: there is no GUI marker distinguishing "uploaded" from "delivered and healthy"
- GUI feature ask: no issue filed yet for a delivery/runtime status column next to a Unit

Pause after this stage.

## Stage 5: Verify The Evidence (read-only)

```bash
./verify.sh
```

This re-renders everything locally and checks the structure: the healthy
overlay's ports, the failed-sync overlay's one added `RedisCache` on top
of an unchanged healthy render, the bad-commit overlay's Service
`targetPort` of 8080 against a container and probes that stay on 80 (both
broken overlays in the healthy app's own namespace), and that every script
in this example (including every `scenarios/*/break.sh`) is syntactically
valid. It does not call
ConfigHub, so it passes even if you skipped Stage 4.

GUI checkpoint:

- GUI now: none for this stage; verification is local
- GUI gap: none identified
- GUI feature ask: none

Pause after this stage.

## Stage 6: Walk The Three Scenarios (read-only, documentation)

Read each scenario page in order. None of these commands are run by you or
by this walkthrough:

- [`scenarios/01-drift/README.md`](./scenarios/01-drift/README.md)
- [`scenarios/02-failed-sync/README.md`](./scenarios/02-failed-sync/README.md)
- [`scenarios/03-bad-commit/README.md`](./scenarios/03-bad-commit/README.md)

For each one, say: what a person sees in Argo CD or Flux, what ConfigHub
shows (or does not show), and the read-only command a person would run,
in their own terminal against their own cluster, to check it.

GUI checkpoint:

- GUI now: none; these are documentation pages
- GUI gap: no dedicated "broken state" view in the GUI for any of the three
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 7: Cleanup

```bash
./cleanup.sh
```

This removes local rendered output under `var/`. It prints, but does not
run, the `cub space delete` commands for the Spaces Stage 4 created.

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
