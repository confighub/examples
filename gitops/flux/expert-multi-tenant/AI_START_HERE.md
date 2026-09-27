# AI Start Here

Use this page when you want to drive `gitops/flux/expert-multi-tenant`
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
Read gitops/flux/expert-multi-tenant/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This is an expert-level Flux multi-tenancy repo: one shared cluster, one
platform-owned bootstrap layer, and three teams, each with its own
namespace, ServiceAccount, RoleBinding, ResourceQuota and NetworkPolicy.
Everything renders offline. This example never runs `flux bootstrap`, never
reconciles anything, and never touches a cluster.

## Facts To Get Right Before You Say Anything

- One cluster: `shared`.
- Three teams: `team-storefront` (Deployment + Service), `team-payments`
  (Deployment + Service), `team-loyalty` (Deployment only).
- The platform bootstrap Kustomization (`clusters/shared/tenants.yaml`) sets
  no `serviceAccountName`. That is deliberate: it is the one layer allowed
  to create RBAC and namespaces for every team.
- Each team's own Kustomization (in `sync.yaml`) sets `serviceAccountName`
  to its own ServiceAccount and `targetNamespace` to its own namespace.
  Those two values must agree with the team's own RoleBinding, or a real
  cluster refuses the apply.
- Each team's `workloads/` folder is not in that team's own
  `kustomization.yaml`. It is reconciled separately, by the Kustomization
  object `sync.yaml` defines.
- This example does not prove that a live cluster returns `Forbidden` for
  the break-it edit. Say that plainly if asked; do not imply it was tested
  live.

## Stage 1: Preview The Plan (read-only)

```bash
cd gitops/flux/expert-multi-tenant
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not touch any cluster.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI surface for the render plan before upload
- GUI feature ask: no issue filed yet for a plan-oriented GUI handoff on this example

Pause after this stage.

## Stage 2: Read The Tenant Boundary Without Running Anything (read-only)

```bash
kustomize build clusters/shared
kustomize build tenants/base/team-storefront
kustomize build tenants/base/team-payments
kustomize build tenants/base/team-loyalty
```

Say out loud, before moving on: which layer runs with the platform's own
identity, which ServiceAccount each team's Kustomization impersonates, and
which namespace each team's RoleBinding actually grants rights in.

GUI checkpoint:

- GUI now: none; this stage reads the repo only
- GUI gap: no view that lines up a team's RoleBinding scope against its
  Kustomization's targetNamespace
- GUI feature ask: no issue filed yet

Pause after this stage.

## Stage 3: Render And Upload (mutates ConfigHub)

```bash
./setup.sh
```

What this mutates:

- creates or updates ConfigHub Space `gitops-flux-multi-tenant-platform`
- creates or updates ConfigHub Spaces
  `gitops-flux-multi-tenant-team-storefront`,
  `gitops-flux-multi-tenant-team-payments` and
  `gitops-flux-multi-tenant-team-loyalty`

What this does not mutate:

- no live cluster, no Flux installation, no Git history

GUI checkpoint:

- GUI now: open the four Spaces in ConfigHub and inspect the uploaded Units;
  notice that each team's Space holds only that team's own bootstrap and
  workloads
- GUI gap: there is no single view across the three team Spaces that shows
  access side by side
- GUI feature ask: no issue filed yet for a cross-team access view

Pause after this stage.

## Stage 4: Verify The Evidence (read-only)

```bash
./verify.sh
```

This re-renders every layer locally and checks the structure: the platform
bootstrap has no impersonation, every team has its own namespace, RBAC,
quota and NetworkPolicy, every team's Kustomization impersonates only its
own ServiceAccount and targets only its own namespace, and every path any
Kustomization points at exists. It does not call ConfigHub, so it passes
even if you skipped Stage 3.

GUI checkpoint:

- GUI now: none for this stage; verification is local
- GUI gap: none identified
- GUI feature ask: none

Pause after this stage.

## Stage 5: Break It On Purpose (optional, local edits only)

The README lists three single-edit breakages: a tenant escape (a
Kustomization's `targetNamespace` pointed at another team), a missing
`serviceAccountName` (the opposite failure: too much access instead of a
refusal), and a removed `NetworkPolicy` (a guardrail quietly gone). Make
one, run `./verify.sh`, and read the failure message it prints: it names
what a live cluster would do (Kubernetes RBAC refuses the apply, Flux marks
the Kustomization not `Ready`) and says plainly that this repo only proves
the offline structural version of that refusal, not the live one. Undo the
edit with `git checkout -- .` before moving on.

## Stage 6: Cleanup

```bash
./cleanup.sh
```

This removes local rendered output under `var/`. It prints, but does not
run, the `cub space delete` commands for the Spaces Stage 3 created.

## Related Files

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [NOTICE](./NOTICE)
