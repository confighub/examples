# Scenario 2: failed sync

A manifest that renders cleanly cannot actually apply, because it depends
on something the cluster does not have yet.

## The change

[`../../apps/apptique/overlays/failed-sync/redis-cache.yaml`](../../apps/apptique/overlays/failed-sync/redis-cache.yaml)
adds one resource next to the healthy app: a `RedisCache` custom resource
for a session cache apptique is about to start using. Its CRD
(`cache.apptique.example`) has not been installed on the cluster this
example is checked against.

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

## What a person sees

- Argo CD: the sync operation fails with an error such as
  `no matches for kind "RedisCache" in version "cache.apptique.example/v1"`.
  The Application's sync status shows `Failed` (or stays `OutOfSync` with a
  sync error), and the other resources in the same sync wave may or may not
  have applied first, depending on ordering.
- Flux: the Kustomization's `Ready` condition goes `False`, with a reason
  such as `ReconciliationFailed` (or `BuildFailed` on older Flux versions),
  and the message names the missing kind. `flux get kustomizations` in the
  person's own terminal shows the failure; this example never runs it.
- The healthy resources in this same manifest (the Deployment, Service, and
  ServiceAccount) may already be running. A failed sync on one resource
  does not necessarily roll back the resources that did apply, which is
  part of why "failed sync" is a state to name precisely rather than a
  single pass/fail flag.

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

# Argo CD's own read of the Application's sync/health state.
argocd app get apptique-broken-states -o json

# Flux's own read of the Kustomization's conditions.
flux get kustomizations apptique-broken-states

# Pilot's read-only six-link trace: ConfigHub Release, controller object,
# and runtime target, named together instead of checked one at a time.
pilot delivery-trace --space gitops-expert-broken-states \
  --cluster-space <cluster-space> --namespace apptique-broken-states \
  --cub-context <context> --out-dir <receipt-dir>
```

None of these mutate anything. This example does not run any of them; they
are what a person runs, against their own cluster, to see the failure this
scenario describes.
