# What's new in cub argo

Each release is tagged `cub-argo-v<version>` in confighub/examples and installs
with `cub plugin install confighub/examples@cub-argo-v<version> --name argo`.

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
