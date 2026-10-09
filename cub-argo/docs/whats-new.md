# What's new in cub argo

Each release is tagged `cub-argo-v<version>` in confighub/examples and installs
with `cub plugin install confighub/examples@cub-argo-v<version> --name argo`.

## 0.3.1, 2026-10-09

Needs ConfigHub v0.8.2 or newer for live status.

- **Live status is recorded where ConfigHub now reads it.** From ConfigHub
  v0.8.2, live status lives on the Release, and the Healthy gate reads the
  newest published release of a Space and nothing else. `cub argo status` wrote
  the Space annotation `confighub.com/live-status`, which a current server does
  not read, so on an estate without argobot the gate never passed. It now
  records each reading on the release whose digest Argo CD synced, in
  ConfigHub's normalized words with Argo's own beside them, as argobot does. A
  reading of an older release is recorded on that release, and the command
  says the gate reads the newest. A reading that names no published release is
  recorded nowhere, and said.
- **A reading that is no longer true is withdrawn.** If the newest release
  holds a passing reading this reporter wrote and Argo CD now reports another
  release, or none, that reading is replaced, so the gate does not go on
  passing on it.
- **Applications with several sources are reported.** Argo CD lists one synced
  revision for each source; the one for the source that reads the Space is
  used. Before, such an Application had no revision to go by.
- **argobot's reading is left alone while it says the same.** argobot writes
  only when something changes, so an old reading of its is not a stopped
  reporter. It is replaced only when it is both old and different.
- **The gate's rule is the server's.** A release passes when it is synced and
  healthy with no operation running or failed. An Application Argo has run no
  sync operation on no longer counts against it.
- **argobot.sh installs an argobot that starts.** It installed argobot v0.1.7,
  which asks ConfigHub for its Targets by worker. From ConfigHub v0.7.0 a
  Target names no worker, so the server refuses the request and argobot exits,
  again and again. It now installs v0.1.9, which finds its Targets by grant.
- **A published release lands at once.** argobot v0.1.9 finds a moved estate's
  Applications by the Space their source reads, so it refreshes them when that
  Space publishes. Measured on a kind cluster with Argo CD v3.5.4: a release
  reached the cluster in under 20 seconds, where without argobot it waited
  157 seconds for Argo's own poll. The note saying releases still wait is gone.
- **Live status reaches the Release.** argobot records it on the Release, which
  takes `EditChildren` on the Target. `apply.sh` now grants it on the Targets
  it creates, and `argobot.sh` grants it on the ones an earlier `apply.sh`
  made, so an estate that is already onboarded needs only `argobot.sh` again.
- **handover.sh takes the address as its help gives it.** The help asks for
  `CONFIGHUB_OCI=oci://<gateway host>` and the script wrote the scheme again,
  so that form gave `oci://oci://…`. It now takes the address with or without
  the scheme, as `move-applications.sh` already did.

## 0.3.0, 2026-10-02

Needs `cub` and ConfigHub v0.7.0 or newer.

- **ConfigHub is asked through the SDK.** `cub argo check`, `check --record`
  and `status` read releases, revisions and Spaces, and write live status and
  attestations, through the ConfigHub SDK (v0.8.0) in the plugin's own process.
  They no longer run `cub` and read what it prints, so they do not need `cub`
  on the PATH and do not break when its output changes. They use the login
  `cub` passes a plugin, or the active context. The scripts `cub argo apply`
  writes still call `cub`: they are for you to read and run.
- **Targets without a worker.** `apply.sh` creates each Target the way `cub`
  v0.7.0 does, with no worker, provider or parameters. `cub argo` 0.2.0 fails
  against `cub` v0.7.0 at that step.
- **The never-delete line names generated Applications.** An ApplicationSet
  gives what it generates the resources finalizer whether or not its template
  names it, so `plan` warns about each of them, and knows the finalizer's
  `/foreground` and `/background` forms.
- **The guide says what repointing means**, with a diff of the one edit and
  what happens to each kind of object.

## 0.2.0, 2026-09-30

- **Live status without argobot.** `cub argo status [--watch] [--hard-refresh]`
  writes what each handed-over Application synced into its Space as
  `confighub.com/live-status`, in argobot's shape: `Synced` only at the newest
  published release's digest, `OutOfSync` naming what differs, `Unknown` until
  Argo has compared its ConfigHub source. `--hard-refresh` asks Argo, once per
  release, to read a release it cached past. A fresh argobot reading is left
  alone.
- **Estates on Argo CD's own cluster.** One that deploys only to `in-cluster`,
  and so has no cluster Secret, plans and onboards. A cluster generator with an
  empty selector includes `in-cluster`, as the ApplicationSet controller does.
- **Plain directories.** A source path with no kustomization is read as Argo
  reads it, recursively with `directory.recurse`, and both scripts render it the
  same way. An Application that names its tool (`directory`, `kustomize`,
  `plugin`) gets it, whatever files are at the path.
- **Plain Applications handed over.** `handover.sh` checks every Application,
  not only generated ones. A child of an app of apps is changed in its Unit (the
  edit is written for you); one applied by hand is repointed by the script. The
  way back restores each whole original source and prints the exact Unit
  restores.
- **The gateway credential is scoped to the estate**
  (`oci://<gateway>/space/<prefix>-`), so two estates on one gateway no longer
  collide, and `handover.sh` refuses a competing Secret.
- **`cleanup.sh` checks for itself.** With `ARGOCD_CONTEXT` it refuses while any
  Application reads a Space it would delete, and it removes the credential.
- **`e2e/run.sh`** runs the whole journey on a kind cluster of its own and
  compares every UID at each move.

## 0.1.0, 2026-09-30

The first release. An Argo CD estate onboards, hands over and reports back:

- **Plan and onboard.** `cub argo plan` reads a repository or a `kubectl get`
  export and shows the estate ConfigHub would govern: one base per
  ApplicationSet, one variant per generated Application, staged by a cluster
  label. `cub argo apply` writes `apply.sh`, which fills ConfigHub while Argo
  carries on reading Git, and `cleanup.sh`, which takes it back out.
- **Hand over without recreating anything.** `handover.sh` points `root` at
  ConfigHub and prints the reviewed edit that does the same for each app of
  apps. Before any workload's source moves, it checks every Application's
  release against the cluster it deploys to: every object Argo owns, and every
  field the release sets. It records each source first and prints the way
  back, leaves first.
- **A delivery object per variant.** `move-applications.sh` makes each
  Application an ApplicationSet generated into a Unit named after its
  variant's Space, reading it, one stage at a time; retired ApplicationSets
  generate nothing more. A cluster that joins later gets its Application from
  what the template renders for it.
- **Status back.** `argobot.sh` runs argobot beside Argo CD as the estate's
  own worker: live status on each variant, and each release landing at once
  (the refresh needs an argobot release with confighub/argobot#14).
- **Evidence.** `cub argo check --fields --record` records each verdict as a
  LiveCheck attestation on the revision it checked.
- **Joins.** Re-running `apply.sh` for a joining cluster leaves every released
  variant as ConfigHub holds it, and stops if the base holds a change one of
  them has not taken.

Run end to end on Argo CD v3.5.3 in kind against ConfigHub v0.6.8: see the
guide's "What has and has not been checked".
