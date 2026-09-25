# Argo beginner: one app, app of apps

This example is a small, realistic Argo CD repo layout for one app running in
two environments, managed with the app-of-apps pattern. It is part of the
`gitops/` canonical example set. See [`../../README.md`](../../README.md) for
the full index.

## Who this is for

You know `kubectl`. You have clicked through the Argo CD web UI. You have one
app deployed to one or two environments, and someone set it up with one root
Application that creates a child Application per environment. You are not
running a fleet yet, and you do not use ApplicationSets, cluster labels, sync
windows, or staged rollout.

## The first question this person asks

"If I hand this repo to a tool, will it actually understand what my app is
and where it goes?"

## What this example shows

- One root Argo CD `Application` (`root/root-app.yaml`) that points at the
  `apps/` directory.
- One child `Application` per environment in `apps/`. Each child points at
  its own directory of plain YAML and deploys it into its own namespace
  (`apptique-dev`, `apptique-prod`), creating the namespace with
  `CreateNamespace=true`.
- Plain YAML per environment in `manifests/apptique/<env>/`: a
  `Deployment`, `Service` and `ServiceAccount`. Prod runs 3 replicas with
  larger requests and limits; dev runs 1. Each environment is a full copy, not
  an overlay.
- One app (`apptique`, a small frontend). This example replaces the former
  `incubator/apptique-argo-app-of-apps`, which was moved here and removed from
  the incubator.

## Repo layout

```
gitops/argo/beginner-app-of-apps/
  root/
    root-app.yaml             # the root Application, points at apps/
  apps/
    apptique-dev.yaml         # child Application: manifests/apptique/dev  -> namespace apptique-dev
    apptique-prod.yaml        # child Application: manifests/apptique/prod -> namespace apptique-prod
  manifests/apptique/
    dev/deployment.yaml       # Deployment, Service, ServiceAccount; 1 replica
    prod/deployment.yaml      # the same shapes; 3 replicas, larger requests and limits
```

Compare it with [`../beginner-applicationset`](../beginner-applicationset/README.md),
which deploys the same app to the same two environments with one
ApplicationSet and a Kustomize base plus overlays. Here each environment is
an explicit child Application and a full copy of its YAML, which is easy to
read and easy to let drift apart.

## What this example does not do

It does not create or manage a live Kubernetes cluster, and it does not
install Argo CD. The root and child Applications are the manifests a real Argo
CD instance would apply; this example does not apply them anywhere. What
`setup.sh` actually does is collect each environment's YAML and upload it into
ConfigHub, so you can inspect and diff the same config Argo CD would deploy,
without needing a cluster.

## Read-only first

```bash
cd gitops/argo/beginner-app-of-apps
./setup.sh --explain
./setup.sh --explain-json | jq
```

Both commands are read-only: no ConfigHub calls, no cluster calls.

## Running it for real

```bash
./setup.sh
./verify.sh
```

`./setup.sh` collects each environment's YAML and uploads it into its own
ConfigHub Space with `cub variant upload`, into the same namespace the child
Application deploys to. This mutates ConfigHub (it creates or updates two
Spaces). It does not touch a live cluster.

## Mutation boundaries

- `./setup.sh --explain` and `./setup.sh --explain-json`: read-only.
- `./setup.sh`: mutates ConfigHub (creates or updates two Spaces). Does not
  mutate live infrastructure.
- `./verify.sh`: read-only. Checks the Applications and the YAML locally;
  does not call ConfigHub.
- `./cleanup.sh`: removes local rendered files. Prints, but does not run,
  the `cub space delete` commands to remove what `setup.sh` created.

## Pilot tasks we check

Once this example is uploaded, these are representative tasks to try against
it:

- Explain what this repo deploys and where: one `apptique` frontend, to
  `apptique-dev` and `apptique-prod`, through a root Application and two child
  Applications.
- Change the replica count in prod only, and show the reviewed diff before
  anything is applied.
- Show whether dev and prod differ, and explain the difference in plain
  language (replica count, requests and limits, and the environment label).
- Find a difference between dev and prod that nobody intended, since each
  environment is a full copy.

## Related examples

- [`../beginner-applicationset`](../beginner-applicationset/README.md): the
  same app and environments, with an ApplicationSet and Kustomize overlays.
- [`../expert-app-of-apps`](../expert-app-of-apps/README.md): app of apps at
  fleet scale, with ApplicationSets, sync waves and a production sync window.
- [`../../personas/argo-beginner.md`](../../personas/argo-beginner.md): the
  persona this example is written for.

## AI-safe path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
