# flux-system (placeholder)

This directory is a placeholder for the files `flux bootstrap` writes when you
point it at a real cluster and repo:

- `gotk-components.yaml`: the Flux controller manifests
- `gotk-sync.yaml`: the `GitRepository` and root `Kustomization` that make
  this cluster follow this repo

This example does not run `flux bootstrap` against a live cluster, so those
generated files are not included here. `infrastructure.yaml` and `apps.yaml`
in the parent directory show the same shape those generated files would
point at.
