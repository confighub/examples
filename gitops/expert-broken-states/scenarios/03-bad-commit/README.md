# Scenario 3: bad commit

A change applies cleanly. GitOps sync reports success. The workload is
broken anyway, and nothing about the sync state says so.

## The change

[`../../apps/apptique/overlays/bad-commit/kustomization.yaml`](../../apps/apptique/overlays/bad-commit/kustomization.yaml)
is the healthy overlay with one field changed: the `frontend` Service's
`targetPort` moves from 80 to 8080. Nothing else changes. The nginx
container still listens on port 80, its readiness and liveness probes
still check port 80, and the overlay renders into the same
`apptique-broken-states` namespace as the healthy app, so it replaces that
app in place, the way a real bad commit does.

This is the shape a bad commit usually takes in a real repo: one field, in
one file, that a reviewer skims past because everything else in the diff
looks routine. A wrong image tag that does not exist, and a Service whose
selector no longer matches after a label rename, are the same shape: the
manifest is well-formed, the sync applies it without complaint, and the
break is not in the shape of the YAML.

```bash
kustomize build ../../apps/apptique/overlays/bad-commit
diff <(kustomize build ../../apps/apptique/overlays/healthy) \
  <(kustomize build ../../apps/apptique/overlays/bad-commit)
```

Both commands are read-only: they only render local files. The diff is one
line.

To see it on a live cluster, point the existing Argo CD Application's
`path` (or the Flux Kustomization's `path`) at
`gitops/expert-broken-states/apps/apptique/overlays/bad-commit` in a copy of
this repo you control. The namespace, Application name and Kustomization
name all stay the same, so the diagnostics below apply unchanged.

## What a person sees

- Argo CD: the Application syncs successfully and reports `Healthy`. The
  Deployment really did roll out: its pods start, pass their probes on
  port 80 and go Ready. Argo's health check for a `ClusterIP` Service does
  not look at whether its `targetPort` reaches anything. `Synced` and
  `Healthy` both show green.
- Flux: the Kustomization's `Ready` condition is `True`. Flux's health
  checks (configured in [`../../flux/apps.yaml`](../../flux/apps.yaml))
  watch the Deployment's rollout status, which succeeds for the same
  reason.
- The actual symptom shows up one layer up: the Service's endpoints are the
  Ready pods on port 8080, where nothing is listening, so requests to the
  `frontend` Service are refused. Nobody sees that from the GitOps side.

## What ConfigHub shows

It depends on whether the bad commit reached ConfigHub as well as Git.

- **If it did** (the optional upload step below, or any pipeline that
  uploads each commit to ConfigHub before a controller delivers it):
  ConfigHub's stored `frontend-service` Unit and the live Service agree
  field for field, both with `targetPort: 8080`, because the wrong port
  was committed, rendered, uploaded and applied consistently. Comparing
  ConfigHub's intent to the delivered state proves nothing wrong here: the
  two agree. This is exactly the class of bug that intent-versus-delivery
  comparison cannot catch by itself; it needs a runtime or
  application-level check (an end-to-end request through the Service, or a
  human noticing the app is down) on top of the sync state, not instead of
  it.
- **If it did not** (this example as `setup.sh` leaves it, because
  `setup.sh` uploads only the healthy overlay): ConfigHub still stores
  `targetPort: 80` while Git delivered 8080. Comparing the two shows a
  difference, but what that difference says is "ConfigHub and Git
  disagree", not "the app is broken". Upload the bad commit and the
  difference disappears while the app stays broken.

Either way the lesson is the same: agreement between intent and delivery
is not proof that the app works.

## Optional: put the bad commit in ConfigHub too (mutates ConfigHub)

This step is not run by `setup.sh`, `verify.sh` or `break.sh`. It is for a
person who wants to see the "if it did" case above in ConfigHub. It mutates
ConfigHub only: it does not publish a Release and does not touch a
cluster. In YOUR terminal, after `./setup.sh`, from
`gitops/expert-broken-states`:

```bash
mkdir -p var
kustomize build apps/apptique/overlays/bad-commit > var/rendered-bad-commit.yaml
cub variant upload --component apptique --variant healthy --environment Healthy \
  --namespace apptique-broken-states --space gitops-expert-broken-states \
  var/rendered-bad-commit.yaml
```

The flags are exactly the ones `setup.sh` uses for the healthy render, so
this lands in the same Space, Component and variant, the way a real bad
commit lands on the same environment. Upload is create-or-update: because
the render differs from the healthy one only in the Service's
`targetPort`, the upload writes a new revision of the one
`frontend-service` Unit and leaves the other Units unchanged. To put
ConfigHub back, run `./setup.sh` again, which re-uploads the healthy render
as another new revision with `targetPort: 80`.

## The way back

Because this is one field, the fix is the same size as the break: revert
the Service's `targetPort` to 80, publish, and let Argo or Flux re-sync. A
ChangeSet-wrapped promotion (as the get-started tutorial's "make a change"
step does) gives this an undo path: `cub changeset create`, patch, promote
with `--changeset`, close it, then publish; `pilot operate undo` can then
reverse the whole change if the fix itself turns out wrong.

## How to diagnose this, read-only

In the person's own terminal:

```bash
# Sync state alone: both look healthy, which is the trap.
argocd app get apptique-broken-states -o json
flux get kustomizations apptique-broken-states

# Where the Service actually sends traffic: Ready pods, on port 8080.
kubectl -n apptique-broken-states get endpointslices \
  -l kubernetes.io/service-name=frontend

# A request-level check the sync state does not perform.
kubectl -n apptique-broken-states run probe --rm -i --restart=Never \
  --image=curlimages/curl -- curl -sS -m 3 http://frontend.apptique-broken-states.svc.cluster.local

# ConfigHub's own stored intent for the Service. It says targetPort 80 if
# only setup.sh has run, and 8080 if the optional upload above has run.
cub unit data --space gitops-expert-broken-states frontend-service
```

None of these mutate anything except the short-lived debug pod the fourth
command creates and removes; this example does not run any of them.
