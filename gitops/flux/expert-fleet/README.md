# Flux expert: a three-cluster fleet with tenants

This example is a Flux fleet repo of the kind a platform team actually runs:
three clusters, layered Kustomizations with dependency ordering, an
infrastructure and apps split, one component delivered as a chart, image
automation on dev only, a tenant with its own service account, and a
promotion path from dev to staging to production written into the layout. It
is part of the `gitops/` canonical example set. See
[`../../README.md`](../../README.md) for the full index.

## Who this is for

You own a Flux fleet repo that several teams commit to. You know
Kustomizations, HelmReleases, sources, `dependsOn`, post-build substitution
and the multi-tenancy pattern with namespaces and service accounts. You have
automation opening pull requests faster than anyone reads them. You are not
looking for a bootstrap tutorial. You want to see whether a tool can read
the fleet you already have and tell you the truth about it.

## The first question this person asks

"Can a tool map my clusters, tenants and sources without changing anything,
and then show me a promotion with its diff, its approval and its way back?"

## What this example shows

- **Three clusters** under `clusters/dev`, `clusters/staging` and
  `clusters/prod`, each with a `flux-system/` placeholder explaining what
  `flux bootstrap` writes there and why it is not committed.
- **Layered Kustomizations with `dependsOn`**: infrastructure first, then
  apps and tenants, then image automation. Each layer names the path it
  reconciles, so the ordering is readable without a cluster.
- **An infrastructure and apps split**: sources and platform components in
  `infrastructure/`, product workloads in `apps/`, tenant workloads in
  `tenants/`.
- **A HelmRelease from a HelmRepository source**: `edge-router` comes from
  the `platform-charts` source rather than from manifests in Git. The
  rendered workload does not exist in this repo, and a tool that reads the
  repo should say so instead of guessing.
- **Image automation for apptique**: an `ImageRepository` watching the
  registry, an `ImagePolicy` that only accepts patch versions of one minor
  release, and an `ImageUpdateAutomation` that writes the chosen tag back
  into `apps/dev` and nowhere else.
- **A tenant**: `team-checkout` has its own namespace, its own
  `ServiceAccount`, its own source, and a `Kustomization` that runs as that
  service account. A change from that team cannot reach the platform layer,
  whatever their repo says.
- **Post-build substitution**: the apptique Deployment carries
  `${cluster_name}` and `${environment}`, and each cluster supplies them in
  its own `Kustomization`. Those values are real on the cluster and absent
  from the manifests, which is one of the places tools quietly get this
  shape wrong.

## The promotion path, written into the layout

| Environment | Branch it reads | How a change gets there |
|---|---|---|
| dev | `main` | Image automation writes the new tag into `apps/dev` and commits it. |
| staging | `main` | A person copies the tag into `apps/staging` in a pull request. |
| prod | `production` | Someone merges `main` into the `production` branch. |

The branch difference is not documentation, it is a one-line patch:
`infrastructure/prod/kustomization.yaml` changes the `fleet-repo`
GitRepository to follow `production`. Dev and staging inherit `main` from
the base.

The image tags in this repo show that path mid-flight on purpose: dev is on
the newest tag, staging one behind, production one behind that.

## Repo layout

```
gitops/flux/expert-fleet/
  clusters/{dev,staging,prod}/
    flux-system/README.md        # what flux bootstrap writes, and why it is not here
    infrastructure.yaml          # layer 1
    apps.yaml                    # layer 2, dependsOn infrastructure, postBuild substitution
    tenants.yaml                 # layer 3, dependsOn infrastructure
    image-automation.yaml        # layer 4, dev cluster only
  infrastructure/
    base/sources/                # GitRepository (fleet) and HelmRepository (charts)
    base/controllers/            # ingress-system namespace and the edge-router HelmRelease
    {dev,staging,prod}/          # per-environment patches, including the production branch
  apps/
    base/apptique/               # shared Deployment, Service, ServiceAccount
    {dev,staging,prod}/          # namespace, replica count, image tag
  image-automation/              # ImageRepository, ImagePolicy, ImageUpdateAutomation
  tenants/
    base/team-checkout/          # rbac.yaml, sync.yaml, and the tenant's own workloads
    {dev,staging,prod}/          # which revision each environment gives the tenant
```

## What a governed tool should be able to say about this repo

If a tool reads this repo and cannot answer all of these, it does not
understand the repo yet:

| Question | Answer this repo gives |
|---|---|
| Which apps are here? | `apptique` (product), `edge-router` (platform, from a chart), `checkout-api` (tenant-owned). |
| Which environments? | `dev`, `staging`, `prod`. |
| Which clusters? | `dev-1`, `staging-1`, `prod-1`, one per environment. |
| Which namespaces? | `flux-system`, `ingress-system`, `apptique-dev`, `apptique-staging`, `apptique-prod`, `team-checkout`. |
| Who owns what? | The platform team owns `infrastructure/`, `apps/`, `clusters/` and the tenant boundary. `team-checkout` owns its workloads folder and nothing above it. |
| What reconciles first? | `infrastructure` on every cluster. `apps` and `tenants` wait for it. `image-automation` waits for `apps`. |
| Which values are not in Git? | `${cluster_name}` and `${environment}`, supplied per cluster by post-build substitution, and everything the `edge-router` chart renders. |
| What writes to Git by itself? | Image automation, into `apps/dev` only. |
| How does a change reach production? | A merge into the `production` branch. Nothing else moves production. |

## What this example does not do

It does not create or manage a live Kubernetes cluster, does not run
`flux bootstrap`, and does not reconcile anything. The `platform-charts`
source is a placeholder that nothing in this example pulls from. What
`setup.sh` actually does is render every layer of every cluster and upload
the result into ConfigHub, so you can inspect and diff the same config Flux
would reconcile, without a cluster.

## Read-only first

```bash
cd gitops/flux/expert-fleet
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
one per environment plus one for the Flux control objects. This mutates
ConfigHub. It does not touch a live cluster.

## Mutation boundaries

- `./setup.sh --explain` and `./setup.sh --explain-json`: read-only.
- `./setup.sh`: mutates ConfigHub (creates or updates four Spaces). Does not
  mutate live infrastructure.
- `./verify.sh`: read-only. Renders locally and checks the output; does not
  call ConfigHub, Flux, or a cluster.
- `./cleanup.sh`: removes local rendered files. Prints, but does not run,
  the `cub space delete` commands for what `setup.sh` created.

## Break it on purpose

Each of these is one edit, and each breaks the fleet in a way a real fleet
breaks. Make the edit, run `./verify.sh`, and see whether the tool you are
testing catches it before Flux does:

1. **Missing substitution.** Remove `cluster_name` from the `postBuild`
   block in `clusters/prod/apps.yaml`. The manifests still render. On a
   cluster, the Kustomization fails to build, and only production is
   affected.
2. **Broken ordering.** Delete the `dependsOn` block from
   `clusters/staging/apps.yaml`. Apps can now reconcile before the source
   they read from exists, which looks like a transient failure that fixes
   itself, until one day it does not.
3. **Tenant escape.** Remove `serviceAccountName: team-checkout` from
   `tenants/base/team-checkout/sync.yaml`. Everything still renders, and the
   tenant's manifests are now applied with the controller's rights.
4. **Promotion skipped.** Change the branch in
   `infrastructure/prod/kustomization.yaml` back to `main`. Production now
   follows every merge, and the promotion gate is gone with no error
   anywhere.
5. **Automation out of range.** Widen the `ImagePolicy` range to `>=1.0.0`.
   Automation can now write a minor upgrade into dev without anyone asking.

## Pilot tasks we check

- Read this repo and say what each cluster and each tenant runs, and which
  source it came from, without changing anything.
- Say which values are missing from the manifests and only appear at
  reconcile time.
- Promote the apptique image tag from dev to staging, showing the diff, the
  clusters it reaches, the approval it needs, and the way back.
- Say what it would take for the same tag to reach production, and who has
  to act.
- Show that a change inside the tenant folder cannot reach the platform
  layer, and say what stops it.

## Attribution

The layout follows two upstream Apache-2.0 Flux examples. No files were
copied from them. See [`NOTICE`](./NOTICE).

## Related examples

- [`../beginner`](../beginner/README.md): the same app, one cluster, no
  tenants.
- [`../../argo/expert-app-of-apps`](../../argo/expert-app-of-apps/README.md):
  the same size of estate, run by Argo CD.
- [`../../personas/flux-expert.md`](../../personas/flux-expert.md): the
  persona this example is written for.

## AI-safe path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
