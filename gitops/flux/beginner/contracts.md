# Contracts

## Read-Only Contracts

### `./setup.sh --explain`

- mutates: no
- output shape: plain text
- stable text anchors: `read-only setup plan`, `ConfigHub mutations if you run without --explain`
- proves: the plan before any ConfigHub mutation

### `./setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`, `mutates_live_infra`, `spaces`, `units`, `evaluation_modes`
- expected anchors:
  - `.example_name == "gitops-flux-beginner"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 2`
  - `.units == ["apptique-dev", "apptique-prod"]`
- proves: the example plan before any mutation

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 2 Spaces (`gitops-flux-beginner-dev`, `gitops-flux-beginner-prod`), one Unit set per Space from the rendered manifests
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete` commands it prints

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-flux-beginner checks passed.`
- proves:
  - `kustomize build` succeeds for `apps/dev`, `apps/prod`, `infrastructure/dev`, and `infrastructure/prod`
  - the dev render contains `Namespace/apptique-dev`, `Deployment/frontend`, and `Service/frontend`
  - the prod render contains the same resources in `apptique-prod`
  - dev and prod replica counts differ
  - the infrastructure render contains the shared `GitRepository/apptique-examples`
  - `clusters/dev` and `clusters/prod` Kustomizations point at this example's paths
- does not require ConfigHub or a live cluster

## Live Rendering Reference

### `kustomize build apps/dev`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the dev overlay renders a `Namespace`, `ServiceAccount`, `Service`, and one-replica `Deployment`

### `kustomize build apps/prod`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the prod overlay renders the same shapes with 3 replicas and larger resource limits

### `kustomize build infrastructure/dev`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the shared `GitRepository/apptique-examples` source renders correctly
