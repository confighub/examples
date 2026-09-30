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
object in the folder above is what they reconcile. It runs as the
controller's own `kustomize-controller` account, not a tenant's, because bootstrapping
namespaces and RBAC for every team is the platform team's job, not any one
team's.

## Controller flags (documentation only)

The tenant boundary in this example needs kustomize-controller (and
helm-controller, if a team uses `HelmRelease`) to run with
`--no-cross-namespace-refs=true` and `--default-service-account=<name>`. See
"Controller flags this depends on" in the example README for why.

After bootstrap writes `gotk-components.yaml`, the usual way to set them is a
patch in the `kustomization.yaml` that bootstrap generates in this folder:

```yaml
patches:
  - patch: |
      - op: add
        path: /spec/template/spec/containers/0/args/-
        value: --no-cross-namespace-refs=true
      - op: add
        path: /spec/template/spec/containers/0/args/-
        value: --default-service-account=default
    target:
      kind: Deployment
      name: "(kustomize-controller|helm-controller)"
```

`default` is the ServiceAccount named `default` in the namespace of each
Kustomization or HelmRelease. Nothing here grants it any rights, so a
Kustomization with no `serviceAccountName` is refused instead of running as
the controller. The platform's `tenants` Kustomization names
`kustomize-controller`, Flux's own cluster-admin account in `flux-system`,
so it is not affected. The patch is not in a real `kustomization.yaml` in this repo,
because that file only exists after bootstrap, so `./verify.sh` neither
applies nor checks it.
