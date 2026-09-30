# Scenario 2: failed sync

A manifest that renders cleanly cannot actually apply, because it depends
on something the cluster does not have yet.

## The change

[`../../apps/apptique/overlays/failed-sync/redis-cache.yaml`](../../apps/apptique/overlays/failed-sync/redis-cache.yaml)
adds one resource next to the healthy app: a `RedisCache` custom resource
for a session cache apptique is about to start using. Its CRD
(`cache.apptique.example`) has not been installed on the cluster this
example is checked against. The overlay builds on the healthy overlay, so
everything else is unchanged and everything, the `RedisCache` included,
renders into the same `apptique-broken-states` namespace.

This is a real and common shape of failure: a platform team's CRD and a
product team's manifest that uses it land in the same review, or close
together, and the manifest reaches the cluster first.

`kustomize build apps/apptique/overlays/failed-sync` renders this
successfully; kustomize does not know or care whether a CRD exists for a
given `apiVersion`/`kind`. Only a live API server does that check.

```bash
kustomize build ../../apps/apptique/overlays/failed-sync
```

That command is read-only and safe to run: it never contacts a cluster or
ConfigHub, it only reads local files with kustomize.

To see it on a live cluster, point the existing Argo CD Application's
`path` (or the Flux Kustomization's `path`) at
`gitops/expert-broken-states/apps/apptique/overlays/failed-sync` in a copy
of this repo you control. The namespace, Application name and
Kustomization name all stay the same, so the diagnostics below apply
unchanged.

## What a person sees

These describe what each controller would report. This example never runs
either one.

- Argo CD: the Application's sync status is `OutOfSync`, because Git now
  holds a `RedisCache` the cluster does not have. The sync operation's
  phase (`status.operationState.phase`) is `Failed`. Argo CD's dry-run
  validation fails the sync before it applies anything, so nothing from the
  broken commit is applied. Objects already live from the healthy state
  keep running. The error would read roughly "the server could not find the
  requested resource", or that it could not find `RedisCache` in
  `cache.apptique.example` and to make sure the CRD is installed on the
  destination cluster. The exact text depends on the Argo CD version. (With
  the `SkipDryRunOnMissingResource=true` sync option, Argo CD skips that
  validation for the missing kind and applies the other resources. This
  Application does not set it.)
- Flux: kustomize-controller runs a server-side dry-run of the whole
  apply, and that dry-run fails, so nothing from the new revision is
  applied. The Kustomization's `Ready` condition goes `False` with reason
  `ReconciliationFailed`, and the message names the missing kind
  (`RedisCache`) and that no matches were found for it. This overlay builds
  cleanly, so this is not a `BuildFailed`. `flux get kustomizations` in the
  person's own terminal shows the failure; this example never runs it.
- Either way, the Deployment, Service and ServiceAccount from the healthy
  state stay as they were, and no `RedisCache` object exists. The failure
  is in delivery of the new commit, not in the running app, which is part
  of why "failed sync" is a state to name precisely rather than a single
  pass/fail flag.

## What ConfigHub shows

If this overlay were uploaded, ConfigHub's stored Unit for `RedisCache`
would hold exactly the resource above: `cub variant upload` never checks
whether a cluster has the matching CRD, because uploading never touches a
cluster. ConfigHub intent looks complete and correct. The gap is entirely
in delivery: the controller could not turn that intent into a live object.
Naming that gap (intent exists, delivery failed, no runtime object exists)
is the difference between "PASS" and a false "PASS" on this class of
failure.

This example's own `setup.sh` never uploads this overlay. It exists only
for local rendering and inspection.

## How to diagnose this, read-only

In the person's own terminal:

```bash
# Whether the CRD in question actually exists on this cluster.
kubectl get crd rediscaches.cache.apptique.example 2>&1 || true

# The healthy resources in the same namespace, which stay as they were.
kubectl -n apptique-broken-states get deployment,service frontend

# Argo CD's own read of the Application's sync/health state.
argocd app get apptique-broken-states -o json

# Flux's own read of the Kustomization's conditions.
flux get kustomizations apptique-broken-states
```

None of these mutate anything. This example does not run any of them; they
are what a person runs, against their own cluster, to see the failure this
scenario describes.

`pilot delivery-trace` is not used here. It walks a ConfigHub Release, its
delivery target and its Application Unit down to Argo CD, and this example
creates no Target or Release. It applies once the Argo side is delivered
from a ConfigHub Release, and it does not apply to the Flux path.
