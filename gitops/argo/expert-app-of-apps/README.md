# Argo expert: app of apps across three clusters

This example is a brownfield Argo CD layout of the kind a platform team
actually runs: a hand-applied root application, a bootstrap folder, sync
waves, an ApplicationSet over a fleet, a sync window on production, and a
child app-of-apps that owns a small product stack. It is part of the
`gitops/` canonical example set. See [`../../README.md`](../../README.md)
for the full index.

## Who this is for

You have run Argo CD in production for a while. You know Applications,
ApplicationSets and their generators, AppProjects, sync waves and sync
windows. You have more than one cluster, more than one team committing, and
a mix of product services and shared platform add-ons. You are not looking
for a tutorial that builds a new setup. You want to see whether a tool can
read the setup you already have.

## The first question this person asks

"Can a tool read my estate as it is, tell me what runs where, and let me
make a fleet-wide change in stages without me losing control of the root
app?"

## What this example shows

- A **root application** in `bootstrap/root-app.yaml`. It is the one object
  a human applies by hand. Everything else comes from it.
- A **bootstrap folder** the root app syncs: `bootstrap/children/`. Its three
  objects carry **sync waves** so they land in order: AppProjects at wave
  -10, the platform ApplicationSet at wave 0, the storefront child app-of-apps
  at wave 10.
- An **ApplicationSet with a matrix generator**: every registered cluster
  crossed with the list of platform add-ons. Three clusters and one add-on
  gives three Applications today. Adding an add-on to the list gives six,
  with no new files.
- Three **registered clusters** in `clusters/`: `dev-1` (canary),
  `staging-1` (secondary), `prod-1` (primary). The `env` and `rollout-phase`
  labels are what the generators select on, and what a staged rollout uses
  to decide who goes first.
- A **sync window** on the `storefront` AppProject that denies syncing to
  the production cluster during working hours, manual syncs included.
- A **child app-of-apps** (`apps-of-apps/storefront/`) that owns exactly two
  apps: `apptique` (the frontend) and `checkout-cache` (the supporting
  service it reads from). The cache syncs at wave 0, the frontend at wave 5,
  so the cache is there before the frontend starts asking for it.
- **Kustomize overlays per environment** for every app, with real
  differences: replica counts, resource limits and quotas.
- A **rollout caught mid-flight**: dev and staging run one image tag,
  production is still on the tag before it. That is the state a real repo is
  in most of the time, and a tool that reads this repo should say so rather
  than reporting "one app, one version".

## The deliberate wrinkle

`checkout-cache` is **a Helm chart wrapped in Kustomize**. The chart is
vendored in the repo at `apps/checkout-cache/base/charts/checkout-cache`,
the base kustomization inflates it, and the overlays patch the rendered
output. Two things follow, and both are real:

1. Argo CD only renders this app if its Kustomize build options include
   `--enable-helm`. That is an instance-wide setting, so a repo that renders
   on your laptop can still fail on a different Argo CD instance.
2. Resources that Kustomize inflates from a chart **do not pick up the
   Kustomize `namespace:` field**. Each overlay sets the namespace on them
   explicitly. Delete those patches and the app quietly lands in whatever
   namespace Argo CD was told to use, which is not always the one the rest
   of the stack is in.

A tool that claims to understand this repo has to get both of these right.

## Repo layout

```
gitops/argo/expert-app-of-apps/
  bootstrap/
    root-app.yaml                   # applied by hand, once
    children/
      projects.yaml                 # two AppProjects, wave -10, prod sync window
      platform-addons-appset.yaml   # ApplicationSet, matrix: clusters x add-ons, wave 0
      storefront-app-of-apps.yaml   # the child app-of-apps, wave 10
  clusters/
    dev-1.yaml staging-1.yaml prod-1.yaml   # registration stubs, no credentials
  apps-of-apps/storefront/
    checkout-cache.yaml             # wave 0, cluster generator
    apptique.yaml                   # wave 5, cluster generator
  apps/
    apptique/base + overlays/{dev,staging,prod}
    checkout-cache/base (vendored chart) + overlays/{dev,staging,prod}
    platform/cluster-baseline/base + overlays/{dev,staging,prod}
```

## What a governed tool should be able to say about this repo

If a tool reads this repo and cannot answer all of these, it does not
understand the repo yet:

| Question | Answer this repo gives |
|---|---|
| Which apps are here? | Three: `apptique`, `checkout-cache`, `cluster-baseline`. |
| Which of them is a product app? | `apptique` and `checkout-cache`. `cluster-baseline` is a platform add-on. |
| Which environments? | `dev`, `staging`, `prod`. |
| Which clusters? | `dev-1` (canary), `staging-1` (secondary), `prod-1` (primary). |
| Which namespaces? | `argocd` for the control objects, `platform-system` for the add-on, `storefront-dev`, `storefront-staging` and `storefront-prod` for the product apps. |
| Which objects are roots, which are generators, which are leaves? | Root: `bootstrap/root-app.yaml`. Generators: the platform matrix ApplicationSet and the two cluster-generator ApplicationSets under `apps-of-apps/storefront`. Leaves: the Kustomize overlays under `apps/`. |
| What is in flight right now? | The apptique image tag: dev and staging are ahead of production. |
| What is not committed here? | Cluster credentials. `clusters/*.yaml` carry labels and a server address only. |
| What would a change to one value touch? | A change to `apps/apptique/base` reaches all three clusters. A change to an overlay reaches one. |

## What this example does not do

It does not create or manage a live Kubernetes cluster, does not install
Argo CD, and does not apply the root application. What `setup.sh` actually
does is render every overlay and the control objects, then upload them into
ConfigHub so you can inspect and diff the same config Argo CD would deploy,
without a cluster.

## Read-only first

```bash
cd gitops/argo/expert-app-of-apps
./setup.sh --explain
./setup.sh --explain-json | jq
```

Both commands are read-only: no ConfigHub calls, no cluster calls.

## Running it for real

```bash
./setup.sh
./verify.sh
```

`./setup.sh` renders everything and uploads it into four ConfigHub Spaces,
one per environment plus one for the Argo control objects. This mutates
ConfigHub. It does not touch a live cluster.

## Mutation boundaries

- `./setup.sh --explain` and `./setup.sh --explain-json`: read-only.
- `./setup.sh`: mutates ConfigHub (creates or updates four Spaces). Does not
  mutate live infrastructure.
- `./verify.sh`: read-only. Renders locally and checks the output; does not
  call ConfigHub, Argo CD, or a cluster.
- `./cleanup.sh`: removes local rendered files. Prints, but does not run,
  the `cub space delete` commands for what `setup.sh` created.

## Break it on purpose

Each of these is one edit, and each breaks the repo in a way a real estate
breaks. Make the edit, run `./verify.sh`, and see whether the tool you are
testing catches it before Argo CD does:

1. **Bad path.** Change the `env` label on `clusters/prod-1.yaml` to
   `production`. The generator now points at
   `apps/apptique/overlays/production`, which does not exist. In a live
   instance the Application goes to a failed sync.
2. **Silent wrong namespace.** Delete the namespace patches from
   `apps/checkout-cache/overlays/prod/kustomization.yaml`. Nothing fails to
   render. The cache just stops being in the same namespace as the frontend.
3. **Dropped ordering.** Remove the sync-wave annotation from
   `apps-of-apps/storefront/checkout-cache.yaml`. The frontend can now start
   before the cache exists, which shows up as a flaky first rollout rather
   than an error.
4. **Drift.** Change the replica count in the rendered output rather than in
   the overlay, the way a hotfix by hand does. With `selfHeal: true` Argo CD
   puts it back, and nothing records that anyone tried.

## Pilot tasks we check

- Read this repo and say what it deploys, to which clusters, in which
  namespaces, without changing anything.
- Name which objects are roots, generators and leaves, and say who owns each.
- Promote the apptique image tag from staging to production, showing the diff,
  the clusters it reaches, the order, and the way back.
- Say what the production sync window means for that promotion.
- Change one platform quota on the canary cluster only, then explain what it
  would take to reach the other two.

## Related examples

- [`../beginner-applicationset`](../beginner-applicationset/README.md): the
  same app, one cluster, no fleet.
- [`../../flux/expert-fleet`](../../flux/expert-fleet/README.md): the same
  size of estate, run by Flux.
- [`../../personas/argo-expert.md`](../../personas/argo-expert.md): the
  persona this example is written for.

## AI-safe path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
