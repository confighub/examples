# Scenario 3: bad commit

A change applies cleanly. GitOps sync reports success. The workload is
broken anyway, and nothing about the sync state says so.

## The change

[`../../apps/apptique/overlays/bad-commit/kustomization.yaml`](../../apps/apptique/overlays/bad-commit/kustomization.yaml)
patches the frontend container to listen on port 8080 instead of 80, and
moves its readiness and liveness probes to the same port so the pod still
starts and stays Ready. The Service in the shared base is untouched: it
still sends traffic to port 80.

This is the shape a bad commit usually takes in a real repo: one field, in
one file, that a reviewer skims past because everything else in the diff
looks routine. A wrong image tag that does not exist, and a Service whose
selector no longer matches after a label rename, are the same shape: the
manifest is well-formed, the sync applies it without complaint, and the
break is not in the shape of the YAML.

```bash
kustomize build ../../apps/apptique/overlays/bad-commit
```

That command is read-only: it only renders local files.

## What a person sees

- Argo CD: the Application syncs successfully and its health check can
  report `Healthy`, because Argo's default health check for a Deployment
  looks at replica and condition status, not at whether a Service can
  reach a pod on the right port. `Synced` and `Healthy` both show green,
  because the pods really did come up and pass their (also-moved) probes.
- Flux: the Kustomization's `Ready` condition is `True`. Flux's health
  checks (configured in [`../../flux/apps.yaml`](../../flux/apps.yaml)) watch the
  Deployment's rollout status, which also succeeds, for the same reason.
- The actual symptom shows up one layer up: requests to the `frontend`
  Service time out or connection-refuse, because nothing is listening on
  port 80 in the pod anymore. Nobody sees that from the GitOps side.

## What ConfigHub shows

ConfigHub's stored Unit matches the live object field for field, because
the wrong port was committed, rendered, uploaded and applied consistently.
Comparing ConfigHub's intent to the delivered state proves nothing wrong
here: the two agree. This is exactly the class of bug that intent-versus-
delivery comparison cannot catch by itself; it needs a runtime or
application-level check (a readiness probe that actually exercises the
real port, an end-to-end request, or a human noticing the app is down) on
top of the sync state, not instead of it.

## The way back

Because this is one field, the fix is the same size as the break: revert
the container port (and its probes) to 80, publish, and let Argo or Flux
re-sync. A ChangeSet-wrapped promotion (as the get-started tutorial's
"make a change" step does) gives this an undo path:
`cub changeset create`, patch, promote with `--changeset`, close it, then
publish; `pilot operate undo` can then reverse the whole change if the fix
itself turns out wrong.

## How to diagnose this, read-only

In the person's own terminal:

```bash
# Sync state alone: both look healthy, which is the trap.
argocd app get apptique-broken-states -o json
flux get kustomizations apptique-broken-states

# A request-level check the sync state does not perform.
kubectl -n apptique-broken-states run probe --rm -i --restart=Never \
  --image=curlimages/curl -- curl -sS -m 3 http://frontend.apptique-broken-states.svc.cluster.local

# ConfigHub's own stored intent, to confirm it matches the delivered
# object (and therefore cannot be the thing that catches this).
cub unit get --space gitops-expert-broken-states --json frontend
```

None of these mutate anything except the short-lived debug pod the third
command creates and removes; this example does not run any of them.
