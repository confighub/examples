# flux-system (placeholder)

On a real cluster, `flux bootstrap` writes `gotk-components.yaml`,
`gotk-sync.yaml` and a `kustomization.yaml` into this folder, plus the
deploy key it created. None of that is committed here, because this example
never bootstraps a cluster.

What matters for reading the repo: on cluster `prod-1` the Flux
controllers run in the `flux-system` namespace, and the `Kustomization`
objects in the folder above are what they reconcile.
