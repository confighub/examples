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
  - `.example_name == "gitops-argo-beginner-applicationset"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 2`
  - `.units == ["apptique-dev", "apptique-prod"]`
- proves: the example plan before any mutation

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 2 Spaces (`gitops-argo-beginner-dev`, `gitops-argo-beginner-prod`), one Unit set per Space from the rendered manifests
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete` commands it prints

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-argo-beginner-applicationset checks passed.`
- proves:
  - `kustomize build` succeeds for both `apps/apptique/overlays/dev` and `apps/apptique/overlays/prod`
  - the dev render contains `Namespace/apptique-dev`, `Deployment/frontend`, and `Service/frontend`
  - the prod render contains the same resources in `apptique-prod`
  - dev and prod replica counts differ
  - `bootstrap/applicationset.yaml` points at this example's overlay path
- does not require ConfigHub or a live cluster

## Live Rendering Reference

### `kustomize build apps/apptique/overlays/dev`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the dev overlay renders a `Namespace`, `ServiceAccount`, `Service`, and one-replica `Deployment`

### `kustomize build apps/apptique/overlays/prod`

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the prod overlay renders the same shapes with 3 replicas and larger resource limits
