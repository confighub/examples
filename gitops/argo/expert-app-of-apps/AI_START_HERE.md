# AI Start Here

Use this page when you want to drive `gitops/argo/expert-app-of-apps`
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
Read gitops/argo/expert-app-of-apps/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This is an expert-level Argo CD estate: a hand-applied root application, a
bootstrap folder with sync waves, an ApplicationSet with a matrix generator
over three registered clusters, a production sync window, and a child
app-of-apps that owns two apps. One of those apps is a Helm chart wrapped in
Kustomize. Everything renders offline. This example never installs Argo CD,
never applies the root application, and never touches a cluster.

## Facts To Get Right Before You Say Anything

- Apps: `apptique`, `checkout-cache`, `cluster-baseline`.
- Environments: `dev`, `staging`, `prod`.
- Clusters: `dev-1` (canary), `staging-1` (secondary), `prod-1` (primary).
- Namespaces: `argocd`, `platform-system`, `storefront-dev`,
  `storefront-staging`, `storefront-prod`.
- Production runs an older apptique image tag than dev and staging. That is
  on purpose, not a mistake.
- `checkout-cache` only renders with `kustomize build --enable-helm`.

## Stage 1: Preview The Plan (read-only)

```bash
cd gitops/argo/expert-app-of-apps
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not touch any cluster.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI surface for the render plan before upload
- GUI feature ask: no issue filed yet for a plan-oriented GUI handoff on this example

Pause after this stage.

## Stage 2: Read The Estate Without Running Anything (read-only)

```bash
kustomize build bootstrap
kustomize build bootstrap/children
kustomize build apps-of-apps/storefront
kustomize build clusters
```

Say out loud, before moving on: which object is the root, which are
generators, which are leaves, and which cluster goes first in a staged
rollout.

GUI checkpoint:

- GUI now: none; this stage reads the repo only
- GUI gap: no view that draws the root, generator and leaf relationship
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 3: Render Every Environment (read-only)

```bash
kustomize build apps/apptique/overlays/prod
kustomize build --enable-helm apps/checkout-cache/overlays/prod
kustomize build apps/platform/cluster-baseline/overlays/prod
```

Note the `--enable-helm` flag on the middle command. Without it the render
fails, and that is the same failure a misconfigured Argo CD instance gives.

GUI checkpoint:

- GUI now: none; local render only
- GUI gap: no side-by-side of what each cluster would receive
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 4: Render And Upload (mutates ConfigHub)

```bash
./setup.sh
```

What this mutates:

- creates or updates ConfigHub Space `gitops-argo-expert-control`
- creates or updates ConfigHub Spaces `gitops-argo-expert-dev`,
  `gitops-argo-expert-staging` and `gitops-argo-expert-prod`

What this does not mutate:

- no live cluster, no Argo CD installation, no Git history

GUI checkpoint:

- GUI now: open the four Spaces in ConfigHub and inspect the uploaded Units
- GUI gap: there is no single fleet view across the three environment Spaces
- GUI feature ask: no issue filed yet for a fleet view over Spaces

Pause after this stage.

## Stage 5: Verify The Evidence (read-only)

```bash
./verify.sh
```

This re-renders everything locally and checks the structure: sync waves, the
sync window, three clusters with three rollout phases, the chart inflation,
the namespaces, and that every path a generator produces exists in the repo.
It does not call ConfigHub, so it passes even if you skipped Stage 4.

GUI checkpoint:

- GUI now: none for this stage; verification is local
- GUI gap: none identified
- GUI feature ask: none

Pause after this stage.

## Stage 6: Break It On Purpose (optional, local edits only)

The README lists four single-edit breakages: a bad generated path, a silent
wrong namespace, dropped sync ordering, and drift. Make one, run
`./verify.sh`, and see what is caught. Undo the edit with
`git checkout -- .` before moving on.

## Stage 7: Cleanup

```bash
./cleanup.sh
```

This removes local rendered output under `var/`. It prints, but does not
run, the `cub space delete` commands for the Spaces Stage 4 created.

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
