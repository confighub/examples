# Drift, failed sync, and bad-commit states

This example is one small healthy app, delivered by Argo CD and by Flux,
plus three documented ways it goes wrong: someone edits the live object by
hand (drift), a manifest that cannot actually apply (failed sync), and a
change that applies cleanly but breaks the app anyway (bad commit). It is
part of the `gitops/` canonical example set. See
[`../README.md`](../README.md) for the full index.

## Where this lives, and why

Every other expert example in this set lives under `gitops/argo/` or
`gitops/flux/`, because it shows one tool's repo shape. This example is not
about a repo shape. It is about three states that show up the same way
whether Argo CD or Flux is delivering the config, so it lives at
`gitops/expert-broken-states/`, next to `argo/` and `flux/` rather than
inside either one. The [index](../README.md) lists it as "Argo CD and
Flux", not one or the other.

## Who this is for

You run Argo CD or Flux (or both) in production. You already know what
`OutOfSync`, a failed sync, and a green dashboard over a broken app look
like on a real cluster. You are not looking for an introduction to GitOps.
You want to know whether a tool can name these three states correctly, tell
you what ConfigHub does and does not add on top of them, and stay
read-only until you ask it to do something else. See
[`../personas/argo-expert.md`](../personas/argo-expert.md) and
[`../personas/flux-expert.md`](../personas/flux-expert.md): this example
is written for both.

## The first question this person asks

"If my live cluster drifts from Git, a sync fails outright, or a change
looks fine but breaks the app, does ConfigHub actually help me see that, or
does it just repeat what Argo or Flux already told me?"

## What this example shows

One app, `apptique`, in one namespace, `apptique-broken-states`, delivered
two ways:

- [`argo/application.yaml`](./argo/application.yaml): an Argo CD
  `Application` with `selfHeal: true`, pointed at
  `apps/apptique/overlays/healthy`.
- [`flux/apps.yaml`](./flux/apps.yaml) and
  [`flux/gitrepository.yaml`](./flux/gitrepository.yaml): a Flux
  `Kustomization` and the `GitRepository` it reads from, pointed at the
  same path, with a health check on the `frontend` Deployment.

And three states of that same app, each its own kustomize overlay so it
renders and can be inspected on its own:

| Overlay | What it is | Renders? | Applies? | Workload actually works? |
|---|---|---|---|---|
| `overlays/healthy` | the only state this example uploads | yes | yes | yes |
| `overlays/failed-sync` | adds a `RedisCache` custom resource with no CRD assumed | yes | no | n/a, never applied |
| `overlays/bad-commit` | the healthy overlay with the Service `targetPort` moved to 8080; the container still listens on 80 | yes | yes | no |

Drift has no overlay of its own, because drift is not a manifest problem:
it is a live object diverging from every manifest that describes it. See
[`scenarios/01-drift/`](./scenarios/01-drift/README.md).

## What this example does not do

It does not create or manage a live Kubernetes cluster, does not install
Argo CD or Flux, and does not apply anything. What `setup.sh` actually does
is render the healthy overlay and the two controllers' control objects,
then upload only the healthy render into ConfigHub, so you can inspect the
same config Argo CD or Flux would deliver, without a cluster. It never
uploads the `failed-sync` or `bad-commit` overlays, and it never runs any
command inside `scenarios/*/break.sh`; those exist so a person can read or
run them, by hand, in their own terminal, against their own cluster.

## Read-only first

```bash
cd gitops/expert-broken-states
./setup.sh --explain
./setup.sh --explain-json | jq
```

Both commands are read-only: no ConfigHub calls, no cluster calls.

## Running it for real

```bash
./setup.sh
./verify.sh
```

`./setup.sh` renders the healthy app and both controllers' control objects,
then uploads them into three ConfigHub Spaces, one per Component: a
ConfigHub Space belongs to one Component, so the Argo control objects, the
Flux control objects, and the healthy app each get their own Space. This
mutates ConfigHub. It does not touch a live cluster, and it does not
upload the broken overlays.

## Mutation boundaries

- `./setup.sh --explain` and `./setup.sh --explain-json`: read-only.
- `./setup.sh`: mutates ConfigHub (creates or updates three Spaces, one per
  Component: an Argo control Space with the Argo Application, a Flux
  control Space with the Flux Kustomization and GitRepository, and a
  workload Space with the healthy `apptique` Unit). Does not mutate live
  infrastructure. Never uploads `overlays/failed-sync` or
  `overlays/bad-commit`.
- `./verify.sh`: read-only. Renders every overlay and both controllers'
  control objects locally and checks the output; does not call ConfigHub,
  Argo CD, Flux, or a cluster.
- `./cleanup.sh`: removes local rendered files. Prints, but does not run,
  the `cub space delete` commands for what `setup.sh` created.
- `scenarios/*/break.sh`: prints documentation, and for the two scenarios
  with a local overlay, also renders that overlay locally with
  `kustomize build` (read-only). None of them touch a cluster or
  ConfigHub, and none of them are run by any script in this example.

## The three scenarios

Each scenario page states what a person sees, what ConfigHub shows, and a
read-only way to check, in the person's own terminal:

1. [**Drift**](./scenarios/01-drift/README.md): a hand edit to a live
   Deployment. Argo CD's `selfHeal` and Flux's drift detection both correct
   it on their next pass; ConfigHub's stored intent never changed, so it
   has no way to notice the divergence by itself.
2. [**Failed sync**](./scenarios/02-failed-sync/README.md): a `RedisCache`
   custom resource with no CRD installed. It renders and uploads cleanly;
   only a live API server rejects it, which is exactly why ConfigHub intent
   alone cannot be read as delivery proof.
3. [**Bad commit**](./scenarios/03-bad-commit/README.md): the frontend
   Service's `targetPort` moves to 8080 while the container still listens
   on 80. Argo and Flux both report success, because the pods really do
   come up and pass their probes on 80. The workload is unreachable
   through its Service anyway. Whether ConfigHub's stored intent agrees
   with the live Service depends on whether the bad commit was also
   uploaded; the scenario page has an optional, person-run upload step
   for that case.

## What a governed tool should be able to say about each state

| Question | Answer this example gives |
|---|---|
| Is `overlays/failed-sync` a kustomize problem or a cluster problem? | Cluster: it renders cleanly, and fails only because the CRD is missing. |
| Does `overlays/bad-commit` fail to sync? | No. Sync and health both report success; the break is a runtime symptom. |
| Does ConfigHub detect drift by itself? | No. Its stored Unit does not change when a live object is hand-edited; something has to read the live cluster and compare. |
| What would still be missing even after a `PASS` on ConfigHub intent? | Delivery/controller state, runtime state, and whichever of those was not actually checked, named explicitly rather than assumed. |

## Pilot tasks we check

- Read this repo and say, for each of the three states, whether it is a
  manifest problem, a delivery problem, or a runtime problem, without
  running anything.
- Name what ConfigHub's stored intent does and does not prove for each
  state.
- Point at the one field that changed in `overlays/bad-commit`, and say why
  sync and health status both stay green anyway.
- Say what a read-only check would need to look at, beyond ConfigHub
  intent, to catch each of the three states.

## Related examples

- [`../argo/expert-app-of-apps`](../argo/expert-app-of-apps/README.md) and
  [`../flux/expert-fleet`](../flux/expert-fleet/README.md): the same
  `apptique` app, at fleet scale, for the same two personas.
- [`../personas/argo-expert.md`](../personas/argo-expert.md) and
  [`../personas/flux-expert.md`](../personas/flux-expert.md): the personas
  this example is written for.

## AI-safe path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
