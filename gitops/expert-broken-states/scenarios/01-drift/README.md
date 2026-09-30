# Scenario 1: drift

Someone edits a live object by hand. Nothing in Git or ConfigHub changes.
The cluster is now different from what both of them say it should be.

## The edit

A person, in a hurry, scales the frontend Deployment up by hand instead of
opening a pull request:

```bash
kubectl -n apptique-broken-states scale deployment/frontend --replicas=5
```

That is the only command in [`break.sh`](./break.sh) in this scenario, and
`break.sh` only prints it. It is not run by this example, and it is not run
by an AI assistant walking through this example. If a human wants to see
the real behavior, they run it themselves, in their own terminal, against
their own cluster.

## What a person sees

The edit above changes `spec.replicas`, a field the manifest sets (to 2),
so both controllers correct it.

- Argo CD: the `apptique-broken-states` Application turns `OutOfSync`. With
  `selfHeal: true` (set in [`../../argo/application.yaml`](../../argo/application.yaml)),
  Argo CD's controller notices on its next reconcile and puts the replica
  count back to the manifest's value. If auto-sync were off, the
  Application would stay `OutOfSync` until someone syncs it by hand, and
  the Argo UI would keep showing the difference.
- Flux: the `apptique-broken-states` Kustomization has `interval: 5m` (see
  [`../../flux/apps.yaml`](../../flux/apps.yaml)). On every interval,
  kustomize-controller applies the desired state again with server-side
  apply. That corrects drift in the fields it manages, so the replica count
  goes back to 2 at the next interval. The Kustomization stays `Ready=True`
  throughout: Flux has Ready conditions, not a separate sync status, so
  the correction shows up only in its events and logs.
- Only fields present in the desired manifest are corrected. A hand edit to
  a field the manifest does not set, such as an annotation someone adds
  with `kubectl annotate`, is not reverted by Argo CD (on a manual sync or
  with `selfHeal`) or by Flux. Neither controller treats that as drift.
- If reconciliation is paused, the cluster keeps running 5 replicas. Argo
  CD still shows `OutOfSync`, because it keeps comparing live state to Git.
  A suspended Flux Kustomization does not re-check, so Flux shows nothing
  new.

## What ConfigHub shows

ConfigHub's stored Unit for `frontend` still has `spec.replicas: 2`, because
uploading the healthy state (`./setup.sh`) never changed. ConfigHub has no
way to know the hand edit happened unless something reads the live cluster
and compares it. ConfigHub intent and cluster runtime state have diverged;
ConfigHub itself does not detect that divergence on its own.

## How to diagnose this, read-only

In the person's own terminal, against their own cluster and their own
ConfigHub context:

```bash
# What ConfigHub believes the desired state is.
cub unit get --space gitops-expert-broken-states --json frontend

# What the cluster is actually running (read-only; needs the kubeconfig
# context for the cluster, a separate binding from the cub context above).
kubectl -n apptique-broken-states get deployment frontend -o json

# Pilot's read-only Day-0 map for the Space.
pilot map --space gitops-expert-broken-states --json
```

Neither `cub unit get` nor `kubectl get -o json` mutates anything. Both are
plain reads. Comparing their two outputs by eye is the manual version of
what a drift check does.

`pilot delivery-trace` is not used here. It walks a ConfigHub Release, its
delivery target and its Application Unit down to Argo CD, and this example
creates no Target or Release. It applies once the Argo side is delivered
from a ConfigHub Release, and it does not apply to the Flux path.
