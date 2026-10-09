# Run log: live status on the Release, 2026-10-09

The expert estate onboarded, handed over and moved, then `cub argo status`
recording each Application's live status on the Release it synced, and a
change released to show a reading following Argo CD from one release to the
next. Kept as a record of what was run and seen.

- **Argo CD:** v3.5.3 on kind, management cluster `rh-fargo`, with
  `kustomize.buildOptions: --enable-helm` and `kustomize.path.v5` in
  `argocd-cm`. One workload cluster, `rh-fdev1` as `dev-1` (canary).
- **ConfigHub:** v0.8.10, hub.confighub.com, gateway `oci.hub.confighub.com`
  over TLS. `cub` v0.8.7.
- **Estate:** this repository's `gitops/argo/expert-app-of-apps`: `root`,
  `storefront`, three ApplicationSets, three generated Applications.
- **Plugin:** built from the branch that records live status on the Release,
  with confighub/examples#290 in it. No argobot.
- **Output:** trimmed to the lines that matter; the digests are the real ones.

## 1. Onboard, hand over, move

`apply.sh`, then `handover.sh` with `CONFIGHUB_OCI=oci://oci.hub.confighub.com`,
the form its help gives: no `oci://oci://` anywhere in what it wrote. This was
also the first run against a gateway over TLS.

```text
dev-1-apptique: 4 objects match what Argo owns
dev-1-checkout-cache: 2 objects match what Argo owns
dev-1-cluster-baseline: 3 objects match what Argo owns
```

`storefront` repointed and the three ApplicationSets retired through their
Units, then `move-applications.sh canary`:

```text
dev-1-apptique reads rh-la-apptique-dev-1 at sha256:f3bc16ed53a69892bdb6ac373db57c1d52717bb0abfbb57344d232f7df0a207a (Synced/Healthy, UID unchanged)
dev-1-checkout-cache reads rh-la-checkout-cache-dev-1 at sha256:b2abf4ee83ca473b62a78442168ae2e1889b5d89c7596633d35d5da86e127713 (Synced/Healthy, UID unchanged)
```

## 2. Each Application's status, on the release it synced

```text
$ cub argo status estate.json --prefix rh-la --stage-label rollout-phase --stages canary --kube-context kind-rh-fargo
root -> rh-la-root-children release 5: Synced/Healthy/Succeeded: release 5 synced (written; the Healthy gate would pass)
storefront -> rh-la-storefront-children release 4: Synced/Healthy/Succeeded: release 4 synced (written; the Healthy gate would pass)
dev-1-apptique -> rh-la-apptique-dev-1 release 1: Synced/Healthy: release 1 synced (written; the Healthy gate would pass)
dev-1-checkout-cache -> rh-la-checkout-cache-dev-1 release 1: Synced/Healthy: release 1 synced (written; the Healthy gate would pass)
dev-1-cluster-baseline -> rh-la-platform-addons-cluster-baseline-dev-1 release 1: Synced/Healthy: release 1 synced (written; the Healthy gate would pass)
```

The moved Applications had run no sync operation of their own, so they carry
no operation; the gate asks only that none is running or failed. What the
Release holds, with Argo CD's own words beside ConfigHub's:

```text
NUM    TAG                                            PUBLISHED    DIGEST          LIVE              CREATED
1      rh-la-apptique-base/onboard-b97387d1-co-end    true         f3bc16ed53a6    Synced/Healthy    2026-10-09 08:37:06

{"DataSource":"dev-1-apptique","Health":"Healthy","Message":"release 1 synced","ObservedAt":"2026-10-09T08:40:18Z","Reporter":"cub-argo","ReporterHealth":"Healthy","ReporterSync":"Synced","Sync":"Synced"}
```

A second pass wrote nothing: all five `unchanged`.

## 3. A reading follows Argo CD to the next release

The frontend was raised to 3 replicas on the base, promoted to canary, approved
and published. Argo CD still held release 1, so that is the release the
reading stayed on, and the newest had none:

```text
dev-1-apptique -> rh-la-apptique-dev-1 release 1: Synced/Healthy: release 1 synced; release 2 is published and Argo CD has not read it: it caches the digest behind latest until a hard refresh (status --hard-refresh asks for one) (written; the Healthy gate would not pass: it reads release 2, the newest, which nothing has reported on)
```

```text
$ cub argo status … --hard-refresh
dev-1-apptique: asked Argo CD for a hard refresh, so it reads the newest release
frontend replicas on dev-1: 3 after about 35s

$ cub argo status …
dev-1-apptique -> rh-la-apptique-dev-1 release 2: Synced/Healthy/Succeeded: release 2 synced (written; the Healthy gate would pass)

NUM    TAG                                            PUBLISHED    DIGEST          LIVE              CREATED
2      rh-la-apptique-base/more-replicas-co-end       true         1b46f5f8b03f    Synced/Healthy    2026-10-09 08:40:30
1      rh-la-apptique-base/onboard-b97387d1-co-end    true         f3bc16ed53a6    Synced/Healthy    2026-10-09 08:37:06
```

## 4. A recorded check judges health

With the build that became 0.3.2. All three Applications Healthy:

```text
$ cub argo check estate.json … --fields --record
dev-1-apptique: release 2 (sha256:1b46f5f8b03f4198035939d58d1bc39545273831e87f82d5ca999d3735febb32) holds apptique at revision 4
  recorded a Pass: LiveCheck attestation f6081b0a-2ac1-4516-90ef-6c1be58640f3 on rh-la-apptique-dev-1/apptique revision 4
…
3 of 3 clean
3 of 3 recorded a Pass

{"Type":"LiveCheck","Result":"Pass","Claims":{"argocd.argoproj.io/application":"dev-1-apptique","argocd.argoproj.io/health":"Healthy","confighub.com/release":"sha256:1b46f5f8b03f4198035939d58d1bc39545273831e87f82d5ca999d3735febb32"},"Note":"cub argo check: 4 objects match what Argo owns, every field the release sets matches on all 4, and Argo CD reports it Healthy"}
```

Then the workload cluster's node was cordoned and the pods in `storefront-dev`
deleted, so two Applications waited for a node. Nothing differed, and nothing
was recorded for them; the command failed:

```text
  recorded nothing yet: Argo CD reports dev-1-apptique as Progressing, which is neither a Pass nor a rejection; check again once that changes
  recorded nothing yet: Argo CD reports dev-1-checkout-cache as Progressing, which is neither a Pass nor a rejection; check again once that changes
  recorded a Pass: LiveCheck attestation 85cc2ffb-8802-4994-b810-5af4a156a9bf on rh-la-platform-addons-cluster-baseline-dev-1/platform-addons-cluster-baseline revision 3
3 of 3 clean
1 of 3 recorded a Pass
exit 1
```

## Not checked

- The Healthy gate refusing a promotion: this estate had one stage. It was
  shown the same day with `cub flux`, against the same server:
  [the Flux run log](../../../cub-flux/docs/runs/2026-10-09-live-status-on-the-release.md).
- A Degraded Application recorded as a rejection: only Progressing was
  produced here. Covered by tests, and run live for a failing Flux layer.
- argobot and `cub argo status` on the same release, an Application with
  several sources, and a reading withdrawn after a rollback. Covered by tests;
  the withdrawal was run live with `cub flux`.
- `cub argo status` run as a worker, which takes `EditChildren` on the Target.

## Afterwards

Both kind clusters and every `rh-la-*` Space were deleted.
