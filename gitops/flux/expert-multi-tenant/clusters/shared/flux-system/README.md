# flux-system (placeholder)

On a real cluster, `flux bootstrap` pointed at this repo with
`--path=gitops/flux/expert-multi-tenant/clusters/shared` writes
`gotk-components.yaml`, `gotk-sync.yaml` and a `kustomization.yaml` into
this folder, plus the deploy key it created. None of that is committed
here, because this example never bootstraps a cluster.

`gotk-sync.yaml` defines a `GitRepository` named `flux-system`, in the
`flux-system` namespace, for the bootstrapped repo, and a `Kustomization`
named `flux-system` that applies `--path` from it. That generated
`flux-system` GitRepository is the source `../tenants.yaml` names in its
`sourceRef`, so the platform layer reads from the same repo and revision
bootstrap set up, and no second source has to be defined. If you bootstrap
with a different `--path` or repo layout, the `path` in `tenants.yaml` is
relative to the root of that repo.

What matters for reading the repo: the Flux controllers run in the
`flux-system` namespace on the shared cluster, and the `Kustomization`
object in the folder above is what they reconcile. It runs with the
controller's own default identity, not a tenant's, because bootstrapping
namespaces and RBAC for every team is the platform team's job, not any one
team's.
