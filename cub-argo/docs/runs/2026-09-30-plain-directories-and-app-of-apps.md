# Run log: an app of apps with plain-directory children, onboarded and handed back, 2026-09-30

The most common Argo CD shape there is: a root Application whose children are
ordinary Applications, each syncing a plain directory of manifests. Before this
run the plugin could not onboard it at all. Kept as a record of what was run
and seen.

- **Argo CD:** v3.5.3 on kind, cluster `cs-argo`.
- **ConfigHub:** v0.6.8, self-hosted, gateway over plain HTTP. `cub` v0.6.8, kustomize v5.8.1.
- **Estate:** this repository's `gitops/argo/beginner-app-of-apps`, syncing from GitHub `main`: root `apptique-apps` syncs `apps/`, which holds `apptique-dev` and `apptique-prod`, each syncing a directory with one `deployment.yaml` and no kustomization.
- **Plugin:** built from this branch, reached as `cub argo` through a shim on `PATH`.
- **Output:** trimmed; digests and counts are the real ones.

## What the plugin could not do before

- `plan` refused both children: "has no kustomization.yaml ... Onboarding a plain directory is not supported yet".
- Had it planned them, `apply.sh` would have failed: a component whose paths name no shared base got no base unit, so each variant was cloned empty and `cub unit update` failed with `unit "apptique-dev" not found`. This run hit exactly that, before the fix.
- Had `apply.sh` passed, `handover.sh` would have checked neither child, moved neither, and said it was done: it handled only ApplicationSets and parents.

## 1. Plan and apply.sh

```text
Argo CD estate: 1 cluster, Argo CD's own (in-cluster), 2 components, 2 variants
Control tree (stays as it is: this is the management record)
  Application apptique-apps                  wave   0  root, applied by hand
    Application apptique-dev                 wave   0  deploys workloads; planned below
    Application apptique-prod                wave   0  deploys workloads; planned below
  note     a plain directory of manifests, read as Argo CD reads it: every .yaml, .yml and .json file at the top level
```

The first `apply.sh` stopped at the empty variant. With the base now made from
the first variant's render, and a missing unit in a variant cloned from the
base, the same `apply.sh` resumed and finished. What ConfigHub stored equals
the plain directory's render byte for byte:

```text
$ diff <(build .../manifests/apptique/dev) <(cub unit data --space cp-apptique-dev-in-cluster apptique-dev)
STORED == RENDERED
```

## 2. handover.sh stopped on a credential another estate left behind

```text
== 2/5 Repoint apptique-apps at cp-apptique-apps-children
  apptique-apps synced latest, not cp-apptique-apps-children's newest release sha256:85eac865...
HANDOVER STOPPED (exit 1) on context kind-cs-argo. These parents read ConfigHub: apptique-apps
```

Argo's condition said why: `401 Unauthorized` from the gateway. The earlier
estate on this cluster had left a `repo-creds` Secret for the whole gateway,
holding a worker `cleanup.sh` had since deleted, and Argo matched it before
the new one. The script stopped where it should, with the root moved and the
way back printed, and nothing pruned. Three changes came of it:

- The credential is scoped to the estate's Spaces, `oci://<gateway>/space/<prefix>-`. Patched on the rig, the root read its control Space at once: Argo takes the longest matching prefix.
- `handover.sh` refuses when another `repo-creds` Secret claims the same prefix.
- `cleanup.sh` deletes the credential `handover.sh` wrote.

## 3. handover.sh, resumed

```text
== 2/4 Repoint apptique-apps at cp-apptique-apps-children
apptique-apps already reads ConfigHub
  apptique-apps reads cp-apptique-apps-children at sha256:85eac865... (Synced; a parent reads OutOfSync until its children move too)
== 3/4 Prove nothing on the clusters would change
  cp-apptique-dev-in-cluster holds what gitops/argo/beginner-app-of-apps/manifests/apptique/dev renders today
apptique-dev: 3 objects match what Argo owns
  and every field the release sets, on all 3 objects, already has that value on the cluster
apptique-prod: 3 objects match what Argo owns
  and every field the release sets, on all 3 objects, already has that value on the cluster
== 4/4 Point each Application at its own Space
-- apptique-dev, apptique-prod: Units in cp-apptique-apps-children, which apptique-apps syncs from ConfigHub
  cub unit update --space cp-apptique-apps-children apptique-dev handover-state/repointed/cp-apptique-apps-children/apptique-dev.yaml --change-desc 'Point apptique-dev at ConfigHub'
  cub unit update --space cp-apptique-apps-children apptique-prod handover-state/repointed/cp-apptique-apps-children/apptique-prod.yaml --change-desc 'Point apptique-prod at ConfigHub'
  cub release publish cp-apptique-apps-children
```

The resumed run also printed a broken way back for the root: its record had
been written by the previous version of the script, in a format the new one
misread. The way back now reads both formats.

## 4. The two printed commands, run as printed

```text
apptique-dev=Synced/Healthy@sha256:263a4968c0391f4e2c8fae4be0da604d6dd4848e6fa4718983b2a95514fbd179
apptique-prod=Synced/Healthy@sha256:c47831f2d02dc00b0a9c06929b43c931ab1ebc915feda6bd6108803ab482f6bc
EVERY UID UNCHANGED (13)
```

Each digest is the one step 3 checked.

```text
$ cub argo status export.yaml --prefix cp --kube-context kind-cs-argo
apptique-apps -> cp-apptique-apps-children: Synced/Healthy/Succeeded at sha256:bb5d397b...: release 2 synced (written; the Healthy gate would pass)
apptique-dev -> cp-apptique-dev-in-cluster: Synced/Healthy/Succeeded at sha256:263a4968...: release 1 synced (written; the Healthy gate would pass)
apptique-prod -> cp-apptique-prod-in-cluster: Synced/Healthy/Succeeded at sha256:c47831f2...: release 1 synced (written; the Healthy gate would pass)
```

## 5. cleanup.sh refuses while anything reads ConfigHub

```text
$ ARGOCD_CONTEXT=kind-cs-argo bash cleanup.sh
These Applications still read a Space this would delete: apptique-apps apptique-dev apptique-prod
Put each source back to Git first (handover.sh printed the way back). Nothing was deleted.
```

Its first attempt at this check could not parse its own jsonpath, and failed
closed: "Could not read the Applications ... Nothing was deleted." Fixed, and a
test now fails on any kubectl template in a generated script that is not closed
on its line.

## 6. The way back, leaves first, then cleanup.sh

Each child's Unit put back from the copy `apply` wrote, one publish, then the
root patched to its recorded source:

```text
apptique-dev=https://github.com/confighub/examples.git|Synced/Healthy apptique-prod=https://github.com/confighub/examples.git|Synced/Healthy
https://github.com/confighub/examples.git|Synced/Healthy
EVERY UID UNCHANGED AFTER THE WAY BACK
```

```text
$ ARGOCD_CONTEXT=kind-cs-argo bash cleanup.sh
== 3/3 The Components those Spaces belonged to, and the credential
secret "confighub-cp-targets" deleted from argocd namespace
== Checking, rather than assuming
  every Space apply.sh made is gone.
```

`handover.sh` now prints those Unit commands itself, with the exact file for
each, rather than "restore the Unit to the revision before".

## 7. Applications applied by hand: intermediate-ci-to-gitops

The same cluster, next: `apptique-dev` and `apptique-prod` applied by hand,
with no parent. Their pods cannot pull the CI-built image the example names, so
Argo holds them at `Progressing`. `handover.sh` now checks and repoints them
itself, as it does a root:

```text
== 2/3 Prove nothing on the clusters would change
apptique-dev: 4 objects match what Argo owns
  and every field the release sets, on all 4 objects, already has that value on the cluster
apptique-prod: 4 objects match what Argo owns
  and every field the release sets, on all 4 objects, already has that value on the cluster
== 3/3 Point each Application at its own Space
application.argoproj.io/apptique-dev patched
  apptique-dev reads ci-apptique-dev-in-cluster at sha256:f5218c15... (Synced; stage fleet)
application.argoproj.io/apptique-prod patched
  apptique-prod reads ci-apptique-prod-in-cluster at sha256:d0d95003... (Synced; stage fleet)
EVERY UID UNCHANGED (16)
```

Status says what is true, and keeps the gate shut:

```text
apptique-dev -> ci-apptique-dev-in-cluster: Synced/Progressing/Succeeded at sha256:f5218c15...: release 1 synced (written; the Healthy gate would not pass)
```

The way back it printed, each whole source, the last moved first, run as
printed; then `cleanup.sh`:

```text
EVERY UID UNCHANGED AFTER THE WAY BACK
secret "confighub-ci-targets" deleted from argocd namespace
  every Space apply.sh made is gone.
```
