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
  - `.example_name == "gitops-argo-beginner-app-of-apps"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 2`
  - `.units == ["apptique-dev", "apptique-prod"]`
- proves: the example plan before any mutation

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 2 Spaces (`gitops-argo-beginner-aoa-dev`, `gitops-argo-beginner-aoa-prod`), one Unit set per Space from the rendered manifests
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete` commands it prints

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-argo-beginner-app-of-apps checks passed.`
- proves:
  - `root/root-app.yaml` points at this example's `apps/` directory
  - `apps/apptique-dev.yaml` and `apps/apptique-prod.yaml` are Applications that point at `manifests/apptique/dev` and `manifests/apptique/prod`, deploy into `apptique-dev` and `apptique-prod`, and set `CreateNamespace=true`
  - each environment's YAML contains a `Deployment`, `Service` and `ServiceAccount` labelled with its environment
  - dev and prod replica counts differ
- does not require ConfigHub or a live cluster

## Rendering Reference

### `manifests/apptique/dev/*.yaml`

- mutates: no
- output shape: Kubernetes YAML stream, used as-is (no Kustomize or Helm)
- proves: the dev child Application deploys a `ServiceAccount`, `Service`, and one-replica `Deployment`

### `manifests/apptique/prod/*.yaml`

- mutates: no
- output shape: Kubernetes YAML stream, used as-is
- proves: the prod child Application deploys the same shapes with 3 replicas and larger requests and limits
