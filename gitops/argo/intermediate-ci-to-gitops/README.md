# Argo intermediate: CI writes image tags, promotion by pull request

This example is a small, realistic pair of repositories: an application repo
whose CI builds an image and proposes a change to a separate GitOps repo,
and the GitOps repo Argo CD actually watches. Promotion to production is a
second pull request, not a rebuild. It is part of the `gitops/` canonical
example set. See [`../../README.md`](../../README.md) for the full index.

## Who this is for

You have one app in one or two environments. It used to be deployed by hand
or from a single repo, and then someone split it: an application repo with
its own CI, and a separate GitOps repo Argo CD watches, because a product
team should not need write access to the thing that reaches production. You
know pull requests and image tags better than you know Argo CD's own
objects. You have not built an ApplicationSet or an app-of-apps.

**A note on persona fit.** The `gitops/personas/` set currently has a
beginner and an expert persona for Argo CD, and this shape sits between them.
[`argo-expert.md`](../../personas/argo-expert.md) lists "separate app and
GitOps repos, CI writes image tags, production changes go through a pull
request" as one bring-shape a production estate can have, but that persona
describes someone running a multi-cluster fleet with ApplicationSets and
sync waves, which this example does not have. [`argo-beginner.md`](../../personas/argo-beginner.md)
does not list a two-repo split among its brought shapes at all. Neither
persona is a good fit for one app, two repos, two pull requests. This
example does not invent a new persona file to cover the gap; see
[`../../personas/README.md`](../../personas/README.md) for the current set.

## The first question this person asks

"My CI already writes to a GitOps repo. If I hand both repos to a tool, will
it understand which pull request did what, and catch a bad promotion before
Argo CD does?"

## What this example shows

- **Two repositories, modeled as two folders**: [`app-repo/`](./app-repo/)
  and [`gitops-repo/`](./gitops-repo/). In a real setup these are two
  separate Git repositories. They are folders here so both render and
  upload together; see the note in each folder's own README.
- **CI writes exactly one line.** `app-repo/.github/workflows/build-and-open-gitops-pr.yaml`
  is illustrative only (see its warning banner; this example never runs it).
  It shows a real build-then-propose flow: build an image, tag it from the
  commit, and open a pull request against `gitops-repo` that changes only
  the dev image tag.
- **Promotion is a second pull request**, not a rebuild and not a branch
  merge. `gitops-repo/.github/workflows/promote-dev-to-prod.yaml` is also
  illustrative only. It copies the tag dev is running into prod and records
  what it promoted in `gitops-repo/environments/apptique/prod/PROMOTED_FROM.md`.
- **Two Argo CD Applications**, one per environment, in
  `gitops-repo/applications/`, each pointing at its own Kustomize overlay
  under `gitops-repo/environments/apptique/`.
- **Prod is one promotion behind dev on purpose.** Dev runs `v1.4.0`; prod
  runs `v1.3.0`. That is the state a real repo is in most of the time, not a
  bug in this example.
- One app (`apptique`, the same fictional frontend the other `gitops/`
  examples use).

## Repo layout

```
gitops/argo/intermediate-ci-to-gitops/
  app-repo/                                      # the application repo
    Dockerfile, src/                             # the app CI builds
    .github/workflows/
      build-and-open-gitops-pr.yaml               # illustrative, never run
  gitops-repo/                                    # the repo Argo CD watches
    applications/
      apptique-dev.yaml                           # Argo Application -> environments/apptique/dev
      apptique-prod.yaml                          # Argo Application -> environments/apptique/prod
    environments/apptique/
      base/                                        # shared Deployment, Service, ServiceAccount
      dev/kustomization.yaml                        # CI's pull request changes this file's image tag
      prod/
        kustomization.yaml                          # a promotion pull request changes this file's image tag
        PROMOTED_FROM.md                             # what the last promotion pull request actually promoted
    .github/workflows/
      promote-dev-to-prod.yaml                      # illustrative, never run
```

## What this example does not do

It does not create or manage a live Kubernetes cluster, and it does not
install Argo CD. The Application objects in `gitops-repo/applications/` are
the manifests a real Argo CD instance would apply; this example does not
apply them anywhere. It does not build a container image, does not push
anything to a registry, and does not open a pull request anywhere; both
workflow files are read as reference material, never executed. What
`setup.sh` actually does is render each environment's Kustomize overlay from
`gitops-repo/` and upload it into ConfigHub, so you can inspect and diff the
same config Argo CD would deploy, without needing a cluster, a registry, or
either repo's CI.

## Read-only first

```bash
cd gitops/argo/intermediate-ci-to-gitops
./setup.sh --explain
./setup.sh --explain-json | jq
```

Both commands are read-only: no ConfigHub calls, no cluster calls, no build,
no pull request.

## Running it for real

```bash
./setup.sh
./verify.sh
```

`./setup.sh` renders `gitops-repo/environments/apptique/dev` and
`gitops-repo/environments/apptique/prod` with `kustomize build`, then uploads
each into its own ConfigHub Space with `cub variant upload`, into the same
namespace its Application deploys to. This mutates ConfigHub (it creates or
updates two Spaces). It does not touch a live cluster, a registry, or
either repo's CI.

## Mutation boundaries

- `./setup.sh --explain` and `./setup.sh --explain-json`: read-only.
- `./setup.sh`: mutates ConfigHub (creates or updates two Spaces). Does not
  mutate live infrastructure.
- `./verify.sh`: read-only. Renders locally and checks the promotion record;
  does not call ConfigHub.
- `./cleanup.sh`: removes local rendered files. Prints, but does not run,
  the `cub space delete` commands to remove what `setup.sh` created.

## Break it on purpose

**Promotion PR merged with the wrong tag.** Edit
`gitops-repo/environments/apptique/prod/kustomization.yaml` and change
`newTag: v1.3.0` to something `PROMOTED_FROM.md` does not say, for example
`v1.3.1` or `v1.4.0`. That single line is exactly what a promotion pull
request changes, and a reviewer who only reads the PR diff sees one string
replaced by another, with no way to tell from the diff alone whether it is
the string that was reviewed.

Run `./verify.sh`. It fails, because it checks the tag in
`kustomization.yaml` against the tag recorded in `PROMOTED_FROM.md`, not
against dev:

```
prod image tag v1.3.1 does not match the promotion record v1.3.0 in
environments/apptique/prod/PROMOTED_FROM.md. Was the wrong tag merged?
```

**What ConfigHub shows that the PR diff does not.** Run `./setup.sh` again
with the bad tag still in place (this mutates ConfigHub; undo the edit
first if you only want the read-only check above). The upload re-renders
`environments/apptique/prod` and writes the result into the prod Space's
Unit. The GitHub pull request diff shows one YAML line changing; the
ConfigHub Unit diff shows the actual container image field that changes as
a result, on the object that would reach the cluster, which is the
difference between reviewing a promotion and reviewing a text edit. Compare
the prod Unit's image field with `environments/apptique/dev`'s to see
whether the promotion actually matches what dev is running, rather than
trusting the line the PR touched.

Undo the edit with `git checkout -- gitops-repo/environments/apptique/prod/kustomization.yaml`
before moving on.

## Pilot tasks we check

- Explain what these two repos deploy and where: one `apptique` frontend, to
  `apptique-dev` and `apptique-prod`, through two Argo Applications, and name
  which repo CI can reach and which it cannot.
- Say which tag each environment is running, and whether that matches what
  the last promotion pull request says it promoted.
- Given an edited `prod/kustomization.yaml`, say whether the merged tag
  matches the promotion record, and show the exact diff before anything is
  applied.
- Show whether dev and prod differ, and explain the difference in plain
  language (image tag, replica count, and requests and limits).

## Related examples

- [`../beginner-app-of-apps`](../beginner-app-of-apps/README.md): the same
  app and environments, one repo, no CI-to-GitOps split.
- [`../intermediate-git-as-database`](../intermediate-git-as-database/README.md):
  a different intermediate failure mode, at fleet scale: the repo itself
  becomes an unreliable operational store, rather than one promotion PR
  carrying a wrong value.
- [`../expert-app-of-apps`](../expert-app-of-apps/README.md): the same
  bring-shape row ("separate app and GitOps repos") at production-fleet
  scale, with ApplicationSets and sync waves.

## AI-safe path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
