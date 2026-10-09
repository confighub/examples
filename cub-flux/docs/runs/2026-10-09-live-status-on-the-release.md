# Run log: live status on the Release, and the Healthy gate, 2026-10-09

A fleet onboarded with `--require Healthy`, one cluster handed over, and a
change released to it: the Healthy gate refusing the next stage until `cub flux
status` had recorded the release's live status, then a rollback to show a
reading that is no longer true being withdrawn. Kept as a record of what was
run and seen, including what went wrong first.

- **Flux:** v2.8.6 on kind, cluster `rh-fflux` as `dev`, installed with `flux
  install`, the layers applied with `kubectl`.
- **ConfigHub:** v0.8.10, hub.confighub.com, gateway `oci.hub.confighub.com`
  over TLS. `cub` v0.8.7.
- **Fleet:** this repository's `gitops/flux/beginner`: two layers,
  `infrastructure` and `apps`; stages `dev`, then `prod`.
- **Plugin:** built from the branch that records live status on the Release.
- **Output:** trimmed to the lines that matter; the digests are the real ones.

## 1. Onboard, hand over, report

`cub flux apply … --require Healthy`, `apply.sh`, then `handover.sh` for `dev`.
Before anything was reported, the release carried no live status:

```text
NUM    TAG                                        PUBLISHED    DIGEST          LIVE    CREATED
1      rh-ls-apps-base/onboard-0bbef832-co-end    true         b1bec90129b6            2026-10-09 08:26:16
```

```text
$ cub flux status gitops/flux/beginner --prefix rh-ls --cluster dev --kube-context kind-rh-fflux
infrastructure -> rh-ls-infrastructure-dev release 1: Synced/Healthy/Succeeded: release 1 applied (written; the Healthy gate would pass)
apps -> rh-ls-apps-dev release 1: Synced/Healthy/Succeeded: release 1 applied (written; the Healthy gate would pass)
```

```text
NUM    TAG                                        PUBLISHED    DIGEST          LIVE              CREATED
1      rh-ls-apps-base/onboard-0bbef832-co-end    true         b1bec90129b6    Synced/Healthy    2026-10-09 08:26:16
```

What the Release holds, and what the Space no longer does:

```text
{"num":1,"live":{"DataSource":"apps","Health":"Healthy","Message":"release 1 applied","ObservedAt":"2026-10-09T08:27:33Z","Operation":"Succeeded","Reporter":"cub-flux","Sync":"Synced"}}
the Space's confighub.com/live-status annotation: absent
```

A second pass wrote nothing (`unchanged`).

## 2. The gate waits for the release that is running

The frontend was raised to 3 replicas on the base, promoted to `dev`, approved
and published. Release 2 had no live status, and promotion to `prod` was
refused, three times, the last after Flux had applied it:

```text
NUM    TAG                                        PUBLISHED    DIGEST          LIVE              CREATED
2      rh-ls-apps-base/more-replicas-co-end       true         9c09029df9e4                      2026-10-09 08:28:03
1      rh-ls-apps-base/onboard-0bbef832-co-end    true         b1bec90129b6    Synced/Healthy    2026-10-09 08:26:16

$ cub variant promote --change-order rh-ls-apps-base/more-replicas --target-stage prod
Failed: Variant 'dev' has no live status for release 2 yet
```

Release 1's passing status did not count: the gate reads the newest release.
This is where a fleet stays with `cub flux` 0.3.0, which writes the Space
annotation and nothing on the Release.

```text
Flux applied: latest@sha256:9c09029df9e46adc3e5390cbb2bf874b3b737ff85e890b3a37544bee446c404c (Ready=True); frontend replicas: 3

$ cub flux status …
apps -> rh-ls-apps-dev release 2: Synced/Healthy/Succeeded: release 2 applied (written; the Healthy gate would pass)

$ cub variant promote --change-order rh-ls-apps-base/more-replicas --target-stage prod
Upgraded 1 unit(s) behind their upstream
```

## 3. A reading that is no longer true is withdrawn

The root was suspended and the `apps` source pinned back to release 1's digest,
so Flux applied release 1 again while release 2 still held Synced/Healthy.

The first run of this left release 2 as it was, and the gate would have gone on
passing: the binary on the rig predated the fix for exactly that, which a
review had found. With the fix built:

```text
$ cub flux status …
apps -> rh-ls-apps-dev release 1: Synced/Healthy/Succeeded: release 1 applied; release 2 is published and not applied yet (unchanged; the Healthy gate would not pass: it reads release 2, the newest, which holds OutOfSync/Unknown: not what is running: Flux reports release 1 from cub-flux (written: the passing reading this reporter left there is withdrawn))

NUM    TAG                                        PUBLISHED    DIGEST          LIVE                 CREATED
2      rh-ls-apps-base/more-replicas-co-end       true         9c09029df9e4    OutOfSync/Unknown    2026-10-09 08:28:03
1      rh-ls-apps-base/onboard-0bbef832-co-end    true         b1bec90129b6    Synced/Healthy       2026-10-09 08:26:16
```

Put back, Flux applied release 2 and the next pass recorded it Synced/Healthy
again.

## Not checked

- A layer Flux is reconciling again at the release it already applied: the
  reporter was not caught mid-pass. Covered by tests.
- A layer handed back to Git, and another reporter's reading on the same
  release. Covered by tests.
- `cub flux status` run as a worker, which takes `EditChildren` on the Target.
  It ran as the signed-in user.

## Afterwards

The kind cluster and every `rh-ls-*` Space were deleted.
