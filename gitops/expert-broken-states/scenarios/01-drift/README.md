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

- Argo CD: the `apptique-broken-states` Application turns `OutOfSync`. If
  `selfHeal: true` is set (it is, in [`../../argo/application.yaml`](../../argo/application.yaml)),
  Argo CD's controller notices on its next reconcile and reverts the
  replica count back to the manifest's value. The window between the hand
  edit and the revert is the only time the drift is visible in the Argo UI.
- Flux: the `apptique-broken-states` Kustomization's drift detection (on by
  default since Flux v2.12) notices the live Deployment no longer matches
  the last-applied manifest, and reconciles it back on its next interval.
  Flux does not distinguish "OutOfSync" from "Unknown" the way Argo does;
  it reports the Kustomization as `Ready=True` once it has corrected the
  drift, with the correction itself visible only in its events and logs
  during the interval it happened.
- Either way, if reconciliation is paused, suspended, or the sync interval
  has not run yet, the cluster keeps running 5 replicas and nothing in
  Argo, Flux, or ConfigHub says so.

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

# A single read-only pass that names ConfigHub intent, delivery/controller
# state, and runtime state together, instead of checking each by hand.
pilot delivery-trace --space gitops-expert-broken-states \
  --cluster-space <cluster-space> --namespace apptique-broken-states \
  --cub-context <context> --out-dir <receipt-dir>
pilot map --space gitops-expert-broken-states --json
```

Neither `cub unit get` nor `kubectl get -o json` mutates anything. Both are
plain reads. Comparing their two outputs by eye is the manual version of
what a drift check does.
