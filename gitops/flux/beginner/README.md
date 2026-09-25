# Flux beginner: one app, standard layout

This example is a small, realistic Flux repo layout for one app running in
two environments. It follows the structure used by the upstream
[`fluxcd/flux2-kustomize-helm-example`](https://github.com/fluxcd/flux2-kustomize-helm-example)
repository. It is part of the `gitops/` canonical example set. See
[`../../README.md`](../../README.md) for the full index.

## Who this is for

You know `kubectl`. You have run `flux bootstrap` at least once, or watched
someone else do it. You have one app deployed to one or two environments
through Flux `Kustomization` objects. You are not running several teams on
one cluster yet, and you do not need post-build variable substitution or
branch-based promotion.

## The first question this person asks

"Does this match the way my team's Flux repo is actually laid out, or is it
a toy?"

## What this example shows

- The standard Flux "clusters, infrastructure, apps" split:
  `clusters/<env>/` holds the Flux `Kustomization` objects that point at
  `infrastructure/<env>/` and `apps/<env>/`.
- A `flux-system/` placeholder in each cluster directory, explaining what
  `flux bootstrap` normally generates there.
- A shared `infrastructure/base/` holding the `GitRepository` source both
  environments read from.
- A standard Kustomize app layout: `apps/base` plus one overlay per
  environment (`apps/dev`, `apps/prod`), patching replica count and resource
  limits only.
- One app (`apptique`, a small frontend). This example replaces the
  earlier `apptique-flux-monorepo` example, which was rebuilt here.

## Repo layout

```
gitops/flux/beginner/
  clusters/
    dev/
      flux-system/            # placeholder: flux bootstrap writes gotk-components.yaml, gotk-sync.yaml here
      infrastructure.yaml     # Flux Kustomization -> infrastructure/dev
      apps.yaml                # Flux Kustomization -> apps/dev, depends on infrastructure
    prod/                      # same shape, pointed at prod
  infrastructure/
    base/
      sources/apptique-examples.yaml   # GitRepository, the shared source
    dev/                        # references base
    prod/                       # references base
  apps/
    base/                       # shared Deployment, Service, ServiceAccount
    dev/                        # namespace apptique-dev, 1 replica
    prod/                       # namespace apptique-prod, 3 replicas, larger limits
```

This example uses a Kustomization-based app, not a HelmRelease. The upstream
reference repo mixes both; a HelmRelease variant is possible future work for
this index and is listed as planned in [`../../README.md`](../../README.md).

## Attribution

The directory names and split between `clusters/`, `infrastructure/`, and
`apps/` follow the structure of
[`fluxcd/flux2-kustomize-helm-example`](https://github.com/fluxcd/flux2-kustomize-helm-example)
(Apache-2.0). No files from that repository were copied; every file here was
written for this example, reusing the app content of the earlier `apptique-flux-monorepo`
example (MIT, this repository's own license). See [`NOTICE`](./NOTICE).

## What this example does not do

It does not create or manage a live Kubernetes cluster, and it does not run
`flux bootstrap`. The `clusters/` directory is reference material for what a
real bootstrapped cluster would follow. What `setup.sh` actually does is
render the Kustomize app overlays and upload them into ConfigHub, so you can
inspect and diff the same config Flux would reconcile, without needing a
cluster.

## Read-only first

```bash
cd gitops/flux/beginner
./setup.sh --explain
./setup.sh --explain-json | jq
```

Both commands are read-only: no ConfigHub calls, no cluster calls.

## Running it for real

```bash
./setup.sh
./verify.sh
```

`./setup.sh` renders both app overlays with `kustomize build` and uploads
each render into its own ConfigHub Space with `cub variant upload`. This
mutates ConfigHub (it creates or updates two Spaces). It does not touch a
live cluster.

## Mutation boundaries

- `./setup.sh --explain` and `./setup.sh --explain-json`: read-only.
- `./setup.sh`: mutates ConfigHub (creates or updates two Spaces). Does not
  mutate live infrastructure.
- `./verify.sh`: read-only. Renders locally and checks the output; does not
  call ConfigHub.
- `./cleanup.sh`: removes local rendered files. Prints, but does not run,
  the `cub space delete` commands to remove what `setup.sh` created.

## Pilot tasks we check

Once this example is uploaded, these are representative tasks to try against
it:

- Explain what this repo deploys and where: one `apptique` frontend, to
  `apptique-dev` and `apptique-prod`, one `Deployment` and `Service` each.
- Change the replica count in prod only, and show the reviewed diff before
  anything is applied.
- Show whether dev and prod differ, and explain the difference in plain
  language (replica count and resource limits, nothing else).
- Roll back a change to prod.

## Related examples

- [`../../argo/beginner-applicationset`](../../argo/beginner-applicationset/README.md):
  the same app, Argo beginner layout.
- [`../../personas/flux-beginner.md`](../../personas/flux-beginner.md): the
  persona this example is written for.

## AI-safe path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
