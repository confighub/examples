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
- stable fields: `example_name`, `mutates`, `mutates_confighub`, `mutates_live_infra`, `spaces`, `units`, `apps`, `environments`, `clusters`, `namespaces`, `evaluation_modes`
- expected anchors:
  - `.example_name == "gitops-argo-expert-app-of-apps"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 4`
  - `.apps == ["apptique", "checkout-cache", "cluster-baseline"]`
  - `.environments == ["dev", "staging", "prod"]`
  - `.clusters == ["dev-1", "staging-1", "prod-1"]`
  - `.units | length == 10`
- proves: the example plan before any mutation

## Structural Contract

A tool may assume all of the following about the files in this directory,
without running anything against a cluster:

- There is exactly one root Application, `bootstrap/root-app.yaml`, and it
  is the only object meant to be applied by hand.
- The root Application syncs `bootstrap/children` and nothing else.
- Every object in `bootstrap/children` carries an
  `argocd.argoproj.io/sync-wave` annotation. The order is `-10` for the
  AppProjects, `0` for the platform ApplicationSet, `10` for the storefront
  child app-of-apps.
- Inside `apps-of-apps/storefront`, `checkout-cache` is wave `0` and
  `apptique` is wave `5`.
- The `storefront` AppProject carries a deny sync window that matches
  applications named `prod-1-*` and sets `manualSync: false`.
- `clusters/` holds exactly three cluster registration stubs, labelled
  `env` (`dev`, `staging`, `prod`) and `rollout-phase` (`canary`,
  `secondary`, `primary`). They carry no credentials.
- Every path a generator produces exists in this repo. A tool may expand
  `{{index .metadata.labels "env"}}` over the three environments and expect
  a `kustomization.yaml` at each result.
- All generators set `goTemplate: true` and `goTemplateOptions:
  [missingkey=error]`, so a missing cluster label is an error, not an empty
  path.
- `apps/checkout-cache` inflates a chart vendored at
  `apps/checkout-cache/base/charts/checkout-cache`. It requires
  `kustomize build --enable-helm`, and its namespace is set by explicit
  patches, not by the Kustomize `namespace:` field.

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 4 Spaces (`gitops-argo-expert-control`, `gitops-argo-expert-dev`,
  `gitops-argo-expert-staging`, `gitops-argo-expert-prod`), one Unit per app
  per environment plus one control Unit
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete` commands
  it prints

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-argo-expert-app-of-apps checks passed.`
- proves:
  - `kustomize build` succeeds for `bootstrap`, `bootstrap/children`,
    `apps-of-apps/storefront` and `clusters`
  - `kustomize build` succeeds for all three overlays of `apptique` and
    `cluster-baseline`, and `kustomize build --enable-helm` for all three
    overlays of `checkout-cache`
  - `helm template` succeeds on the vendored chart on its own
  - each environment renders into its own namespace, and the chart-inflated
    resources carry that namespace too
  - dev and prod differ in both replica count and image tag
  - the sync waves, the deny sync window and `missingkey=error` are present
  - three clusters are registered with three distinct rollout phases
  - every path a generator produces exists on disk
- does not require ConfigHub, Argo CD, or a live cluster

## Live Rendering Reference

### `kustomize build --enable-helm apps/checkout-cache/overlays/prod`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the vendored chart inflates to a `Deployment` and a `Service` in
  `storefront-prod`, with 3 replicas and a 1Gi memory limit

### `kustomize build apps/apptique/overlays/prod`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the prod overlay renders a `Namespace`, `ServiceAccount`,
  `Service` and a 6-replica `Deployment` on the older image tag
