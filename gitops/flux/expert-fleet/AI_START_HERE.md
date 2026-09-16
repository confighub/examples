# AI Start Here

Use this page when you want to drive `gitops/flux/expert-fleet` safely with
an AI assistant.

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
Read gitops/flux/expert-fleet/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This is an expert-level Flux fleet: three clusters, four layers with
dependency ordering, an infrastructure and apps split, a HelmRelease from a
chart source, image automation on dev only, one tenant with its own service
account, and a promotion path from dev to staging to production written into
the layout. Everything renders offline. This example never runs
`flux bootstrap`, never reconciles anything, and never touches a cluster.

## Facts To Get Right Before You Say Anything

- Clusters: `dev-1`, `staging-1`, `prod-1`, one per environment.
- Apps: `apptique` (product), `edge-router` (platform, from a chart),
  `checkout-api` (owned by the tenant).
- Namespaces: `flux-system`, `ingress-system`, `apptique-dev`,
  `apptique-staging`, `apptique-prod`, `team-checkout`.
- Dev and staging read branch `main`. Production reads branch `production`.
- `${cluster_name}` and `${environment}` do not exist in the manifests. Each
  cluster supplies them through post-build substitution.
- What the `edge-router` chart renders is not in this repo. Do not guess it.
- Image automation writes to `apps/dev` only.

## Stage 1: Preview The Plan (read-only)

```bash
cd gitops/flux/expert-fleet
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not touch any cluster.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI surface for the render plan before upload
- GUI feature ask: no issue filed yet for a plan-oriented GUI handoff on this example

Pause after this stage.

## Stage 2: Map The Fleet Without Running Anything (read-only)

```bash
kustomize build clusters/dev
kustomize build clusters/staging
kustomize build clusters/prod
```

Say out loud, before moving on: which layer reconciles first on each
cluster, which cluster has a fourth layer and why, and which values each
cluster supplies that the manifests do not carry.

GUI checkpoint:

- GUI now: none; this stage reads the repo only
- GUI gap: no view of layers and their dependency order
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 3: Read The Promotion Path (read-only)

```bash
kustomize build infrastructure/dev | grep -A3 "kind: GitRepository"
kustomize build infrastructure/prod | grep -A3 "kind: GitRepository"
grep -r "newTag" apps/*/kustomization.yaml
```

Dev follows `main`, production follows `production`, and the three image
tags are deliberately different. Explain what has to happen for the dev tag
to reach production, and who has to do it.

GUI checkpoint:

- GUI now: none; local render only
- GUI gap: no view that lines up the same app across environments
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 4: Read The Tenant Boundary (read-only)

```bash
kustomize build tenants/prod
```

Point at the `serviceAccountName` field and say what would change if it were
missing. Do not propose any change to the tenant's folder in this stage.

GUI checkpoint:

- GUI now: none
- GUI gap: no tenant-ownership view
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 5: Render And Upload (mutates ConfigHub)

```bash
./setup.sh
```

What this mutates:

- creates or updates ConfigHub Space `gitops-flux-expert-fleet`
- creates or updates ConfigHub Spaces `gitops-flux-expert-dev`,
  `gitops-flux-expert-staging` and `gitops-flux-expert-prod`

What this does not mutate:

- no live cluster, no Flux installation, no Git history

GUI checkpoint:

- GUI now: open the four Spaces in ConfigHub and inspect the uploaded Units
- GUI gap: there is no single fleet view across the three environment Spaces
- GUI feature ask: no issue filed yet for a fleet view over Spaces

Pause after this stage.

## Stage 6: Verify The Evidence (read-only)

```bash
./verify.sh
```

This re-renders every cluster and every layer locally and checks the
structure: dependency ordering, post-build substitution, the two sources and
the HelmRelease, the promotion branches, the three image tags, the image
automation scope, and the tenant boundary. It does not call ConfigHub, so it
passes even if you skipped Stage 5.

GUI checkpoint:

- GUI now: none for this stage; verification is local
- GUI gap: none identified
- GUI feature ask: none

Pause after this stage.

## Stage 7: Break It On Purpose (optional, local edits only)

The README lists five single-edit breakages: a missing substitution, broken
ordering, a tenant escape, a skipped promotion gate, and an over-wide image
policy. Make one, run `./verify.sh`, and see what is caught. Undo the edit
with `git checkout -- .` before moving on.

## Stage 8: Cleanup

```bash
./cleanup.sh
```

This removes local rendered output under `var/`. It prints, but does not
run, the `cub space delete` commands for the Spaces Stage 5 created.

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [NOTICE](./NOTICE)
