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

## 4. A layer that starts failing, on a second rig the same day

Run with the build that became 0.3.2, after a review said `cub flux status`
0.3.1 would misread a failed pass. The gateway was given as
`CONFIGHUB_OCI=oci://oci.hub.confighub.com`, and nothing wrote the scheme twice.

The frontend's node was cordoned and its pod deleted, with the `apps` layer's
timeout shortened to 40s. While Flux was checking health, at the release it had
already applied, it said nothing new, and nothing was written:

```text
{"type":"Reconciling","status":"True","reason":"Progressing"}  {"type":"Ready","status":"Unknown","reason":"Progressing"}
apps -> rh-lf-apps-dev: Flux is reconciling the release it already applied; what is recorded stands until it finishes
```

When the pass failed, Flux left the layer not ready and also reconciling, since
it will retry. That is the state 0.3.1 took for a pass still under way:

```text
{"type":"Reconciling","status":"True","reason":"ProgressingWithRetry"}  {"type":"Ready","status":"False","reason":"HealthCheckFailed"}
apps -> rh-lf-apps-dev release 1: OutOfSync/Degraded/Failed: health check failed after 40.014591528s: timeout waiting for: [Deployment/apptique-dev/frontend status: 'InProgress'] (written; the Healthy gate would not pass)

NUM    TAG                                        PUBLISHED    DIGEST          LIVE                  CREATED
1      rh-lf-apps-base/onboard-e4388183-co-end    true         b3ab9137b0f0    OutOfSync/Degraded    2026-10-09 09:19:02
```

`cub flux check --fields --record` on the failing layer recorded a rejection,
though every object and field still matched:

```text
apps: release 1 (sha256:b3ab9137b0f0264c03010e50c4252a4204619dd6a43b7694e1def5e9462700b1) holds apps at revision 3
  recorded a rejection: LiveCheck attestation 4cbb3626-3fd9-462f-8755-a6e7f82136e9 on rh-lf-apps-dev/apps revision 3
  health: Flux reports apps failed: health check failed after 40.014591528s: timeout waiting for: [Deployment/apptique-dev/frontend status: 'InProgress']
apps: 4 objects match what the layer applied
```

With the node uncordoned, Flux's next pass succeeded and the reading went back
to Synced/Healthy/Succeeded.

## 5. fluxbot: the reporter on the cluster, on a third rig

With the build that became 0.4.0, served to the pod from the workstation in
place of a GitHub release. The fleet was onboarded with `--require Healthy` and
`dev` handed over; then:

```text
$ FLUX_CONTEXT=kind-rh-fflux CLUSTER=dev CONFIGHUB_URL=https://hub.confighub.com bash onboard/fluxbot.sh
serviceaccount/fluxbot created
role.rbac.authorization.k8s.io/fluxbot created
rolebinding.rbac.authorization.k8s.io/fluxbot created
deployment.apps/fluxbot created
deployment "fluxbot" successfully rolled out
fluxbot runs in flux-system as rh-fb-targets/server-worker.

$ kubectl -n flux-system logs deploy/fluxbot
apps -> rh-fb-apps-dev release 1: Synced/Healthy/Succeeded: release 1 applied (written; the Healthy gate would pass)
confighub-root -> rh-fb-dev-layers release 1: Synced/Healthy/Succeeded: release 1 applied (written; the Healthy gate would pass)
infrastructure -> rh-fb-infrastructure-dev release 1: Synced/Healthy/Succeeded: release 1 applied (written; the Healthy gate would pass)
```

The init container fetched the binary for the node and its checksum matched.
The pod signed in as the worker, and the grant `fluxbot.sh` made let it write.

A change was released to dev, and promotion to prod asked for every ten
seconds. Nobody ran `cub flux status`:

```text
t=0s   Failed: Variant 'dev' has no live status for release 2 yet
…      (eight refusals)
t=76s  Failed: Variant 'dev' has no live status for release 2 yet
t=87s  Adding 0 unit(s) from upstream at the change order's start

NUM    TAG                                        PUBLISHED    DIGEST          LIVE              CREATED
2      rh-fb-apps-base/more-replicas-co-end       true         94622ed6d72d    Synced/Healthy    2026-10-09 10:20:48
1      rh-fb-apps-base/onboard-86af82b1-co-end    true         140f92c448fe    Synced/Healthy    2026-10-09 10:18:36
```

What the pod said of `apps` meanwhile, one line per change:

```text
apps -> rh-fb-apps-dev release 1: Synced/Healthy/Succeeded: release 1 applied; release 2 is published and not applied yet (written; the Healthy gate would not pass: it reads release 2, the newest, which nothing has reported on)
apps -> rh-fb-apps-dev: Flux is reconciling the release it already applied; what is recorded stands until it finishes
apps -> rh-fb-apps-dev release 2: Synced/Healthy/Succeeded: release 2 applied (written; the Healthy gate would pass)
```

A review of the script then asked for three things, each run on the same rig
once it was changed:

- **Run a second time**, it left the pod as it was: the same pod, no restart.
- **Given a wrong address**, the first version still said "successfully rolled
  out": Kubernetes counted the pod ready the moment it started, before it
  failed. The pod is now ready only once it has signed in, so:

```text
error: timed out waiting for the condition
fluxbot did not come up. What pod/fluxbot-76759b96db-z68jz said, its fetch and then the reporter:
Error: signing in to ConfigHub at https://hub.confighub.example as worker 411ed5f7-…: failed to make authentication request: Post "https://hub.confighub.example/auth/worker": dial tcp: lookup hub.confighub.example on 10.96.0.10:53: no such host
The reporter it was to replace, if there was one, is still running. Run this again once that is put right.

fluxbot-5ccdd7684c-t9x8g 1/1 Running restarts=0; fluxbot-76759b96db-h52l7 0/1 CrashLoopBackOff restarts=5
```

- **With the node restarted**, the fetch ran again over the binary it had left
  and finished, and the reporter carried on.

Once 0.4.0 was published, the test install was taken out with the one
`kubectl delete` the script prints, and `fluxbot.sh` from the released plugin
run with no `FLUXBOT_URL`: the pod fetched `cub-flux-linux-arm64` from the
GitHub release, its checksum matched, and it reported as before.

```text
from=${FLUXBOT_URL:-https://github.com/confighub/examples/releases/download/cub-flux-v0.4.0}
deployment "fluxbot" successfully rolled out
the binary in the pod: cub flux 0.4.0 (3cdc771a816223146707fd4bb648a512fb1bc31d, 2026-10-09T10:51:35Z)
```

The reporter's account was asked what it may do: get and list
`Kustomization`s and `OCIRepository`s in `flux-system`, yes; Secrets, pods, a
patch to a `Kustomization`, or anything in another namespace, no.

## Not checked

- A layer waiting on a dependency. Covered by tests.
- A layer handed back to Git, and another reporter's reading on the same
  release. Covered by tests.
- fluxbot on a cluster that cannot reach github.com, and two clusters each
  with a fluxbot of its own.

## Afterwards

Both kind clusters and every `rh-ls-*` and `rh-lf-*` Space were deleted.
