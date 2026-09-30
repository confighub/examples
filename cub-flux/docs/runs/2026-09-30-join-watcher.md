# Run log: a cluster joins through `cub flux watch`, 2026-09-30

A kept record of what was run and seen.
- **ConfigHub:** v0.6.8, self-hosted. `cub` v0.6.8.
- **Flux:** v2.8.6 on kind.
- **Fleet:** a copy of `gitops/flux/beginner` (clusters `dev`, `prod`), onboarded with `apply.sh` under the prefix `rh-w`.
- **Output:** trimmed to the lines that matter.

## 1. dev-2 joins the fleet in Git

```text
$ cp -R clusters/dev clusters/dev-2 && git commit -m "dev-2 joins"
$ cub flux watch gitops/flux/beginner --prefix rh-w --out onboard --once
dev-2 is proposed: its variants are made, and its release waits for a person to approve it in ConfigHub
  cub variant approve --change-order rh-w-infrastructure-base/onboard-0dc16eb0 --stage dev-2
  cub variant approve --change-order rh-w-apps-base/onboard-962d88f2 --stage dev-2
```

From `onboard/watch.log`:

```text
rh-w-infrastructure-dev already released
rh-w-infrastructure-prod already released
rh-w-infrastructure-dev-2 waits for approval: cub variant approve --change-order rh-w-infrastructure-base/onboard-0dc16eb0 --stage dev-2
rh-w-apps-dev-2 waits for approval: cub variant approve --change-order rh-w-apps-base/onboard-962d88f2 --stage dev-2
  rh-w-dev-2-layers waits: not every variant of dev-2 is released yet
```

Looking again without an approval published nothing: dev-2 had 0 releases.

## 2. A person approves; the watcher releases

```text
$ cub variant approve --change-order rh-w-infrastructure-base/onboard-0dc16eb0 --stage dev-2
$ cub variant approve --change-order rh-w-apps-base/onboard-962d88f2 --stage dev-2
$ cub flux watch gitops/flux/beginner --prefix rh-w --out onboard --once
dev-2 is ready: its layers Space rh-w-dev-2-layers is released. Bring it on with
  FLUX_CONTEXT=<its context> CLUSTER=dev-2 CONFIGHUB_OCI=<gateway> bash onboard/join.sh
```

The results in ConfigHub:
- `rh-w-infrastructure-dev-2`, `rh-w-apps-dev-2` and `rh-w-dev-2-layers` each had 1 release.
- `rh-w-dev-2-layers` carried `flux.confighub.com/joined: proposed by cub flux watch; layers released 2026-09-30T12:37:10Z`.

## 3. join.sh on a Flux-only kind cluster

```text
  infrastructure Ready at latest@sha256:2cbb8be5c9d1...
  apps Ready at latest@sha256:f750eadbdea4...
frontend   1/1     1            1           25s
infrastructure -> rh-w-infrastructure-dev-2: Synced/Healthy/Succeeded at sha256:2cbb8be5c9d1: release 1 applied
apps -> rh-w-apps-dev-2: Synced/Healthy/Succeeded at sha256:f750eadbdea4: release 1 applied
```

## Found by this run, and fixed

**After the approval, the watcher still published nothing.** `apply.sh` counted an onboarding change order as done once it read `Completed`. That only means it was promoted through every stage: a run with `PROPOSE_ONLY`, or one that stopped between promoting and publishing, leaves it `Completed` with a variant unreleased. It now also needs every variant Space in the plan to have a release.

Everything was removed with the generated `cleanup.sh`, and the kind cluster was deleted.
