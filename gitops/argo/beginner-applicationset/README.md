# Argo beginner: one app, ApplicationSet

This example is a small, realistic Argo CD repo layout for one app running in
two environments. It is part of the `gitops/` canonical example set. See
[`../../README.md`](../../README.md) for the full catalog.

## Who this is for

You know `kubectl`. You have clicked through the Argo CD web UI. You have one
app deployed to one or two environments, maybe by hand, maybe through an
ApplicationSet someone else set up. You are not running an app-of-apps fleet
yet, and you do not need cluster labels, sync windows, or staged rollout.

## The first question this person asks

"If I hand this repo to a tool, will it actually understand what my app is
and where it goes?"

## What this example shows

- One Argo CD `ApplicationSet` that uses a **directory generator**: it looks
  at `apps/apptique/overlays/*` and creates one Argo `Application` per
  directory it finds.
- A standard Kustomize layout: a shared `base` plus one overlay per
  environment (`dev`, `prod`). The overlays patch replica count and resource
  limits; nothing else differs between environments.
- One app (`apptique`, a small frontend). This example replaces the former
  `incubator/apptique-argo-applicationset` and `incubator/apptique-flux-monorepo`
  examples, which were moved here and removed from the incubator.

## Repo layout

```
gitops/argo/beginner-applicationset/
  bootstrap/
    applicationset.yaml       # the Argo ApplicationSet, directory generator over overlays/*
  apps/apptique/
    base/                     # shared Deployment, Service, ServiceAccount
    overlays/
      dev/                    # namespace apptique-dev, 1 replica
      prod/                   # namespace apptique-prod, 3 replicas, larger limits
```

This is the same shape Argo CD's own ApplicationSet docs recommend for
"one app, many environments, discovered from directories": no per-environment
copy-paste, one generator finds every environment automatically.

## What this example does not do

It does not create or manage a live Kubernetes cluster, and it does not
install Argo CD. `bootstrap/applicationset.yaml` is the manifest a real Argo
CD instance would apply; this example does not apply it anywhere. What
`setup.sh` actually does is render the Kustomize overlays and upload them
into ConfigHub, so you can inspect and diff the same config Argo CD would
deploy, without needing a cluster.

## Read-only first

```bash
cd gitops/argo/beginner-applicationset
./setup.sh --explain
./setup.sh --explain-json | jq
```

Both commands are read-only: no ConfigHub calls, no cluster calls.

## Running it for real

```bash
./setup.sh
./verify.sh
```

`./setup.sh` renders both overlays with `kustomize build` and uploads each
render into its own ConfigHub Space with `cub variant upload`. This mutates
ConfigHub (it creates or updates two Spaces). It does not touch a live
cluster.

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

- [`../../flux/beginner`](../../flux/beginner/README.md): the same app, Flux
  beginner layout.
- [`../../personas/argo-beginner.md`](../../personas/argo-beginner.md): the
  persona this example is written for.

## AI-safe path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
