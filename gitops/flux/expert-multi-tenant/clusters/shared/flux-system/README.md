# flux-system (placeholder)

On a real cluster, `flux bootstrap` writes `gotk-components.yaml`,
`gotk-sync.yaml` and a `kustomization.yaml` into this folder, plus the
deploy key it created. None of that is committed here, because this example
never bootstraps a cluster.

What matters for reading the repo: the Flux controllers run in the
`flux-system` namespace on the shared cluster, and the `Kustomization`
object in the folder above is what they reconcile. It runs with the
controller's own default identity, not a tenant's, because bootstrapping
namespaces and RBAC for every team is the platform team's job, not any one
team's.
