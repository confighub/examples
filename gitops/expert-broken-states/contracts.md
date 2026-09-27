# Contracts

What a tool may assume about this example.

## Read-Only Contracts

### `./setup.sh --explain`

- mutates: no
- output shape: plain text
- stable text anchors: `read-only setup plan`, `ConfigHub mutations if you run without --explain`
- proves: the plan before any ConfigHub mutation

### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`, `mutates_live_infra`,
  `spaces`, `units`, `apps`, `scenarios`, `uploaded_states`, `not_uploaded_states`,
  `namespaces`, `evaluation_modes`
- expected anchors:
  - `.example_name == "gitops-expert-broken-states"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 3`
  - `.scenarios == ["drift", "failed-sync", "bad-commit"]`
  - `.uploaded_states == ["healthy"]`
  - `.not_uploaded_states == ["failed-sync", "bad-commit"]`
- proves: the example plan before any mutation, and which states this
  example does and does not upload

## Structural Contract

A tool may assume all of the following about the files in this directory,
without running anything against a cluster:

- `apps/apptique/base` renders a `Deployment`, `Service` and
  `ServiceAccount`, all named `frontend`, at 2 replicas, listening on port
  80, with a Service that targets port 80.
- `apps/apptique/overlays/healthy` is the only state this example uploads.
  It sets namespace `apptique-broken-states` and changes nothing else.
- `apps/apptique/overlays/failed-sync` adds exactly one resource beyond the
  healthy render: a `RedisCache` custom resource
  (`cache.apptique.example/v1`) with no matching CRD assumed to exist on
  any cluster this example is checked against.
- `apps/apptique/overlays/bad-commit` builds on the healthy overlay and
  changes exactly one value in its render: the `frontend` Service's
  `targetPort` moves from 80 to 8080. The container, its `containerPort`
  and both probes stay on port 80, and the namespace stays
  `apptique-broken-states`, so the pods go Ready and the rollout succeeds
  while the Service sends traffic to a port nothing listens on.
- `argo/application.yaml` is an Argo CD `Application` with
  `syncPolicy.automated.selfHeal: true`, pointed at
  `gitops/expert-broken-states/apps/apptique/overlays/healthy`.
- `flux/apps.yaml` is a Flux `Kustomization` pointed at the same path, with
  a health check on the `frontend` Deployment. `flux/gitrepository.yaml` is
  the `GitRepository` it reads from.
- All three overlays (`healthy`, `failed-sync`, `bad-commit`) render
  successfully with `kustomize build`. None of them fail to render; the
  difference between them is what a live API server or a live workload
  does with the result, not whether kustomize can build it.

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 3 Spaces (`gitops-expert-broken-states-argo-control`,
  `gitops-expert-broken-states-flux-control`,
  `gitops-expert-broken-states`), one per Component: a ConfigHub Space
  belongs to one Component, so the Argo control objects, the Flux control
  objects, and the healthy `apptique` Unit each get their own Space rather
  than sharing one
- uploads only the healthy overlay; never uploads `failed-sync` or
  `bad-commit`
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete`
  commands it prints

Exact commands this uploads with:

```bash
cub variant upload --component argo-control --variant control --environment Control \
  --namespace argocd --space gitops-expert-broken-states-argo-control \
  var/rendered-argo-control.yaml

cub variant upload --component flux-control --variant control --environment Control \
  --namespace flux-system --space gitops-expert-broken-states-flux-control \
  var/rendered-flux-control.yaml

cub variant upload --component apptique --variant healthy --environment Healthy \
  --namespace apptique-broken-states --space gitops-expert-broken-states \
  var/rendered-healthy.yaml
```

`--component` is required by `cub variant upload --help` on the reviewed
cub v0.6.2. `--namespace` has no default on that same help text, so every
command above states one explicitly rather than relying on a value the
rendered resources happen to already carry.

A ConfigHub Space belongs to one Component: uploading a second Component
into an already-linked Space re-links that Space's ComponentID to the new
Component and overwrites its labels (Owner, Environment among them), which
is why this example never puts the Argo and Flux control objects in the
same Space even though both are "control" objects.

### Optional bad-commit upload (person-run, documented in `scenarios/03-bad-commit/README.md`)

- run by: a person, in their own terminal. No script in this example runs it.
- mutates: yes (ConfigHub only: no Release is published and no cluster is touched)
- writes: a new revision of the `frontend-service` Unit in Space
  `gitops-expert-broken-states`, with `targetPort: 8080`. The other Units
  are unchanged, because the bad-commit render differs from the healthy
  render only in that field.
- uses the same `--component apptique --variant healthy --environment
  Healthy --namespace apptique-broken-states --space
  gitops-expert-broken-states` flags as `setup.sh`, on
  `var/rendered-bad-commit.yaml`
- undo: run `./setup.sh` again, which re-uploads the healthy render as a
  new revision with `targetPort: 80`

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-expert-broken-states checks passed.`
- proves:
  - `kustomize build` succeeds for `argo/`, `flux/`, and all three
    `apps/apptique/overlays/*` directories
  - the healthy overlay's Deployment has 2 replicas and a container and
    Service that both use port 80
  - the failed-sync overlay renders the same Deployment and Service plus
    exactly one `RedisCache` resource
  - the bad-commit overlay renders into `apptique-broken-states`, keeps its
    container and both probes on port 80, sets the Service `targetPort` to
    8080, and differs from the healthy render in exactly one line
  - `setup.sh --explain-json` is valid JSON with the fields and anchors
    listed above
  - `bash -n` passes on every script in this example, including
    `scenarios/*/break.sh`
- does not require ConfigHub, a live cluster, Argo CD, or Flux

## Scenario Contracts

Each scenario under `scenarios/` documents what a person sees, what
ConfigHub shows, and a read-only diagnostic path. None of the three is
proof of live controller behavior; each README says which parts are this
repo's own offline render and which parts describe what Argo CD, Flux, or
ConfigHub would do on a live cluster, and each `break.sh` only prints or
performs a local, read-only kustomize render. No `scenarios/*/break.sh`
mutates ConfigHub or a cluster, and none is run by this example's own
scripts.

| Scenario | Local artifact | Live claim, named as such |
|---|---|---|
| `01-drift` | none (drift is inherently a live divergence) | Argo `OutOfSync`/selfHeal revert; Flux drift detection revert |
| `02-failed-sync` | `apps/apptique/overlays/failed-sync/redis-cache.yaml` | Argo sync error; Flux `Ready=False` |
| `03-bad-commit` | `apps/apptique/overlays/bad-commit/kustomization.yaml` patch | Argo `Synced`/`Healthy`; Flux `Ready=True`; workload still broken |
