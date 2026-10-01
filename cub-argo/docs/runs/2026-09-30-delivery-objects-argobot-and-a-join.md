# Run log: each generated Application a Unit, argobot, and a cluster joining, 2026-09-30

The expert estate handed over, then each Application its ApplicationSets
generated moved onto a Unit reading its own Space, stage by stage; argobot run
as the estate's own worker; a third cluster joined after the retirement; and the
findings of two reviews checked on the same rig. Kept as a record of what was
run and seen, including what went wrong first.

- **Argo CD:** v3.5.3 on kind, management cluster `rh-argo`, installed from the
  release manifest, with `kustomize.buildOptions: --enable-helm` and
  `kustomize.path.v5` in `argocd-cm`, which the example needs.
- **Clusters:** `rh-dev1` as `dev-1` (canary) and `rh-stg1` as `staging-1`
  (secondary), registered with the example's labels; later `rh-prd1` as
  `prod-1` (primary).
- **ConfigHub:** v0.6.8, self-hosted, gateway `192.168.97.2:32181` over plain
  HTTP. `cub` v0.6.8, kustomize v5.8.1.
- **Estate:** this repository's `gitops/argo/expert-app-of-apps`, its `root`
  applied by hand and syncing from GitHub `main`: `root`, the `storefront` app
  of apps, three ApplicationSets (`apptique`, `checkout-cache`,
  `platform-addons`), six generated Applications.
- **Plan input:** a live `kubectl get` export, cluster Secrets with `.data.config`
  and the `last-applied-configuration` annotation removed.
- **Output:** trimmed to the lines that matter; digests and UIDs are the real ones.

## 1. Onboard and hand over

`apply.sh` filled ConfigHub, and `handover.sh` stopped once, as it should:

```text
== 2/5 Repoint root at rh-argo-root-children
root could not read rh-argo-root-children:
application repo oci://192.168.97.2:32181/space/rh-argo-root-children is not permitted in project 'platform'
HANDOVER STOPPED (exit 1) on context kind-rh-argo. These parents read ConfigHub: root
```

`root` syncs the AppProjects from Git with selfHeal, so their `sourceRepos` can
be widened for good only where `root` reads them. The `projects` Unit was edited
and published, the live AppProjects patched long enough for `root` to read it,
and `handover.sh` run again. Every variant matched what Argo owns, field for
field:

```text
dev-1-apptique: 4 objects match what Argo owns
  and every field the release sets, on all 4 objects, already has that value on the cluster
staging-1-checkout-cache: 2 objects match what Argo owns
  and every field the release sets, on all 2 objects, already has that value on the cluster
```

`storefront` was repointed through its Unit, and the three ApplicationSets
retired through theirs (`applicationsSync: create-only`).

## 2. The first move, and what it found

`move-applications.sh canary` made the three canary Units, and both parents
applied them, but the check after it failed:

```text
  storefront applied rh-argo-storefront-children at sha256:2bc796ff…
  dev-1-apptique reads https://github.com/confighub/examples, not its Space rh-argo-apptique-dev-1
STOPPED (exit 1) on context kind-rh-argo.
```

The parent had replaced each Application with its Unit (`dev-1-apptique
replaced`), and the ApplicationSet controller then logged `updated Application`
for each, putting back the template's source and its own ownerReference.
`create-only` stops a controller updating the Applications it owns; one that no
longer carries the ownerReference is, to it, one it has yet to create. Setting
each ApplicationSet's generators to `[{list: {elements: []}}]`, through its
Unit, ended it: `create-only` never deletes, so every Application stayed.
`handover.sh` now prints that edit too, and `move-applications.sh` refuses to
start without it.

## 3. Canary and secondary, every UID kept

```text
== Stage secondary
  staging-1-apptique is a Unit in rh-argo-storefront-children, reading rh-argo-apptique-staging-1
  storefront applied rh-argo-storefront-children at sha256:6fc2260dc091ecf068d16169c32da3718a381a2e8f4e4dc40602e6c0aa45f23b
  root applied rh-argo-root-children at sha256:6602234d55730e37d0a17c38529f523ca18d300a4be11b0f8c4fc7ec026b7493
  staging-1-apptique reads rh-argo-apptique-staging-1 at sha256:35153b83914119dd0b7803777d5d3c7ca173879a8056ad77a5e863c202bae3f9 (Synced/Healthy, UID unchanged)
  staging-1-checkout-cache reads rh-argo-checkout-cache-staging-1 at sha256:6bc781c6a1d7e14c5da09c249b053438867e23ab861e81cdf71dd3698d89e58d (Synced/Healthy, UID unchanged)
  staging-1-cluster-baseline reads rh-argo-platform-addons-cluster-baseline-staging-1 at sha256:52cafeca60e33fa5c826c839618cf3a79fbdd1c1a9060273d8a2a2fcae3ac95f (Synced/Healthy, UID unchanged)
```

Every Application, and every Deployment, Service and ConfigMap the estate owns
on `dev-1` and `staging-1`, had the UID it had before the move. `checkout-cache`
synced too: its template sets `source.kustomize.version: v5`, and the replace
took it off, which a merge would not have.

## 4. The way back, for one Application

```text
after Unit removed: exists uid=e3c72d54-9fa4-4012-bd39-4bf4266b3aaf (was e3c72d54-9fa4-4012-bd39-4bf4266b3aaf); root says OutOfSync requiresPruning=true
patched back: https://github.com/confighub/examples Synced/Healthy uid=e3c72d54-9fa4-4012-bd39-4bf4266b3aaf
30s later (retired generator leaves it): https://github.com/confighub/examples own=
```

`Prune=false` kept `dev-1-cluster-baseline` when its Unit went. Moving it again
picked up where it was.

## 5. argobot, as the estate's own worker

Built from confighub/argobot#14 and run beside the rig with the Targets' server
worker, as `argobot.sh` installs it. Its first start failed on a `409` from
ConfigHub creating three event cursors at once (confighub/argobot#15); the
second ran. It wrote live status for all eight Applications, the moved ones
found by their source:

```text
rh-argo-apptique-dev-1: {"source":"argobot","app":"dev-1-apptique","syncStatus":"Synced","healthStatus":"Healthy","revision":"sha256:28c60fe5…"}
```

A replica change, promoted, approved and published to canary:

```text
published sha256:1c1fd96bf461908447ed497443ce8cbe0da0dfc2ebff8505ef7a590c687dd547 at 17:23:45
[INFO] argobot: release.published (space=a4356acc-… cursor=808) → syncing Argo app "dev-1-apptique"
dev-1 frontend replicas=3 after 2s; uid ef68e5ce-d724-474f-b33e-6f3940be686e
staging-1 frontend replicas: 2 (not promoted there)
```

Promoted to secondary, it reached `staging-1` in 2 seconds too. Without argobot,
Argo was still serving the previous release 90 seconds later. After about half
an hour argobot stopped again on the same `409`.

## 6. A cluster joins after the retirement

`prod-1` was registered with its labels. Nothing was generated for it. A plan
with its Secret added, and `apply.sh` run again, failed first:

```text
Failed: HTTP 400 … unit apptique: the targets are at different revisions of it (2, 3)
```

The replica change was still in flight, released to canary only. Finished
through secondary, `apply.sh` ran, and `move-applications.sh primary` made the
three Applications. The two in the `storefront` project waited for the
example's deny window on `prod-1-*` (09:00 UTC, 9h) and synced when it closed;
`frontend` came up 6/6.

Two defects showed here, both fixed:

- `move-applications.sh` had made the Units before the variants had a release.
  Every Space in a stage is now checked first.
- The re-run of `apply.sh` re-rendered `dev-1` and `staging-1` from Git and
  released them, so both lost the replica change (`head=5`, replicas back to 1
  and 2, release 3 out). A released variant is now left alone once
  `handover.sh` has begun:

```text
heads and releases unchanged:
rh-argo-apptique-dev-1 head=5 release=3
rh-argo-apptique-staging-1 head=5 release=3
rh-argo-apptique-prod-1 head=3 release=1
```

`prod-1-checkout-cache` was then taken out and made again from what its
template renders for `prod-1` (`apps/rh-argo-checkout-cache-prod-1.yaml`):

```text
{"name":"prod-1-checkout-cache","project":"storefront","dest":{"namespace":"storefront-prod","server":"https://rh-prd1-control-plane:6443"},"labels":{"rollout-phase":"primary"},"source":{"path":".","repoURL":"oci://192.168.97.2:32181/space/rh-argo-checkout-cache-prod-1","targetRevision":"latest"},"opts":"Prune=false,Replace=true"}
Synced/Progressing 20f2dca6-e53d-4180-a25b-2560e28fe4f5
```

## 7. Review findings, checked on the rig

- **A change order cannot skip stages.** Narrowed with `--in-scope-space` to
  `prod-1`'s variant, the server refused: `unable to promote to stage 'canary',
  it selects no Space`, then `its previous stage 'secondary' selects no Space`.
  So a join's first release goes through every stage, and `apply.sh` now stops
  when the base holds a change a released variant has not taken:

```text
apply exit 1 (want 1)
rh-argo-apptique-base holds a change rh-argo-apptique-dev-1 has not taken. A joining cluster's first release goes through every stage, so this run would promote that change into dev-1 and approve it. Finish that change,
apply after undo exit 0 (want 0)
```

- **`Replace=true` only for the move.** After it, the Unit and the live
  Application carry `Prune=false` alone; each moved Application's
  `status.history` was already empty, which is what a replace on every parent
  sync does.

## Afterwards

Every `rh-argo-*` Space and the four kind clusters were deleted; the hub went
back to the Spaces other sessions own.
