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
- stable fields: `example_name`, `mutates`, `mutates_confighub`, `mutates_live_infra`, `spaces`, `units`, `apps`, `environments`, `clusters`, `namespaces`, `tenants`, `promotion`, `evaluation_modes`
- expected anchors:
  - `.example_name == "gitops-flux-expert-fleet"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 4`
  - `.environments == ["dev", "staging", "prod"]`
  - `.clusters == ["dev-1", "staging-1", "prod-1"]`
  - `.tenants == ["team-checkout"]`
  - `.units | length == 10`
- proves: the example plan before any mutation

## Structural Contract

A tool may assume all of the following about the files in this directory,
without running anything against a cluster:

- Each of `clusters/dev`, `clusters/staging` and `clusters/prod` holds the
  Flux `Kustomization` objects for that cluster and nothing else. The
  `flux-system/` folder inside each is a placeholder with no Flux objects
  in it.
- Layer order is fixed: `infrastructure` has no dependency, `apps` and
  `tenants` both `dependsOn` `infrastructure`, and `image-automation`
  `dependsOn` `apps`.
- `image-automation` exists on the dev cluster only.
- Every `Kustomization` path in this example points at a directory in this
  repo that contains a `kustomization.yaml`.
- The `apps` Kustomization on each cluster supplies `cluster_name` and
  `environment` through `postBuild.substitute`. The rendered apptique
  Deployment contains the literal strings `${cluster_name}` and
  `${environment}`; they are resolved by Flux, not by Kustomize.
- `infrastructure/base` holds exactly two sources: the `fleet-repo`
  GitRepository and the `platform-charts` HelmRepository. The only
  HelmRelease is `edge-router`. What that chart renders is not in this repo.
- `infrastructure/prod` patches `fleet-repo` to follow the `production`
  branch. `infrastructure/dev` and `infrastructure/staging` inherit `main`.
- The apptique image tag is different in all three environments, newest in
  dev. The dev overlay carries an `$imagepolicy` setter marker; staging and
  prod do not.
- The tenant `team-checkout` has its own Namespace, ServiceAccount,
  RoleBinding, GitRepository and Kustomization. That Kustomization always
  sets `serviceAccountName: team-checkout` and
  `targetNamespace: team-checkout`.

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 4 Spaces (`gitops-flux-expert-fleet`, `gitops-flux-expert-dev`,
  `gitops-flux-expert-staging`, `gitops-flux-expert-prod`), one Unit per
  layer per environment plus one control Unit
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete` commands
  it prints

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-flux-expert-fleet checks passed.`
- proves:
  - `kustomize build` succeeds for all three clusters, all three
    environments of `infrastructure`, `apps` and `tenants`, the
    `image-automation` folder and the tenant's own workloads
  - each cluster orders its layers with `dependsOn`
  - image automation is present on dev and absent on staging and prod
  - each cluster supplies `cluster_name` and `environment`, and the rendered
    app still carries the unresolved `${cluster_name}`
  - the infrastructure layer carries both sources and the HelmRelease
  - dev and staging follow `main`, prod follows `production`
  - the three image tags differ, and only dev carries the setter marker
  - every tenant Kustomization runs as the tenant ServiceAccount and targets
    the tenant namespace
  - every path any Kustomization points at exists in this repo
- does not require ConfigHub, Flux, or a live cluster

## Live Rendering Reference

### `kustomize build clusters/prod`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: three Flux Kustomizations for the production cluster, ordered by
  `dependsOn`, with `cluster_name: prod-1`

### `kustomize build infrastructure/prod`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the fleet source follows the `production` branch, and the
  `edge-router` HelmRelease asks for 3 replicas
