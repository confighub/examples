# Contracts

Stable command outputs for `gitops/argo/intermediate-ci-to-gitops`.

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
  - `.example_name == "gitops-argo-intermediate-ci-to-gitops"`
  - `.mutates == false`
  - `.mutates_confighub == true`
  - `.mutates_live_infra == false`
  - `.spaces | length == 2`
  - `.units == ["apptique-dev", "apptique-prod"]`
- proves: the example plan before any mutation

## Mutating Contract

### `./setup.sh`

- mutates: yes (ConfigHub only, no live infrastructure)
- creates: 2 Spaces (`gitops-argo-ci-to-gitops-dev`, `gitops-argo-ci-to-gitops-prod`), one Unit set per Space from the rendered Kustomize overlay
- does not: build a container image, push to a registry, or open a pull request in either repo
- cleanup: `./cleanup.sh` (local files) plus the `cub space delete` commands it prints

## Verification Contract

### `./verify.sh`

- mutates: no
- output shape: plain text
- stable success text: `All gitops-argo-intermediate-ci-to-gitops checks passed.`
- proves:
  - `gitops-repo/applications/apptique-dev.yaml` and `apptique-prod.yaml` are
    Applications that point at `gitops-repo/environments/apptique/dev` and
    `.../prod`, deploy into `apptique-dev` and `apptique-prod`, and set
    `CreateNamespace=true`
  - each environment's rendered output contains a `Deployment`, `Service` and
    `ServiceAccount` labelled with its environment
  - dev and prod replica counts differ, and dev and prod image tags differ
  - the prod image tag matches the `promoted_tag` recorded in
    `gitops-repo/environments/apptique/prod/PROMOTED_FROM.md`
- does not require ConfigHub or a live cluster

### Break-it check

- what: edit `gitops-repo/environments/apptique/prod/kustomization.yaml`'s
  `newTag` to a value `PROMOTED_FROM.md` does not record
- mutates: no (the edit is a local file change the person makes; `verify.sh`
  itself does not mutate anything)
- output shape: plain text, exits non-zero
- stable text anchor: `does not match the promotion record`
- proves: a promotion pull request that merges the wrong tag is caught
  locally, before Argo CD would ever sync it

## Rendering Reference

### `gitops-repo/environments/apptique/dev/*` (via `kustomize build`)

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the dev Application deploys a `ServiceAccount`, `Service`, and
  one-replica `Deployment` running `ghcr.io/confighub/apptique-frontend:v1.4.0`

### `gitops-repo/environments/apptique/prod/*` (via `kustomize build`)

- mutates: no
- output shape: Kubernetes YAML stream
- proves: the prod Application deploys the same shapes with 3 replicas,
  larger requests and limits, and
  `ghcr.io/confighub/apptique-frontend:v1.3.0`, one promotion behind dev

### `app-repo/.github/workflows/build-and-open-gitops-pr.yaml`, `gitops-repo/.github/workflows/promote-dev-to-prod.yaml`

- mutates: no (illustrative only; never executed by this example or by
  GitHub Actions on this repo, since neither path is this repo's top-level
  `.github/workflows/`)
- output shape: GitHub Actions YAML
- proves: the shape of a real build-then-propose CI step and a real
  promotion step, without this example running either
