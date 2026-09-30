# What's new in cub flux

Each release is tagged `cub-flux-v<version>` in confighub/examples and installs
with `cub plugin install confighub/examples@cub-flux-v<version> --name flux`.

## 0.2.0, unreleased

- **Plain layers onboard.** A layer path with no kustomization is read as
  kustomize-controller reads it: every `.yaml` and `.yml` below it, a
  subdirectory with its own kustomization taken whole. `apply.sh` and
  `handover.sh` render it the same way; before, the plan passed it and
  `apply.sh` failed. The plan names any file there that is not Kubernetes YAML.
- **Layers whose clusters share no base.** The base starts from the first
  cluster's render, so each variant has a unit to update, and a partial first
  run resumes.
- **`cleanup.sh` checks for itself.** Given `FLUX_CONTEXT` or `FLUX_CONTEXTS`,
  it reads each cluster's OCIRepositories and refuses while any reads a Space it
  would delete, and removes the pull Secret once it has deleted them.
- **The way back is complete.** It now removes the OCIRepositories the root
  applied, last; before, they stayed behind reading ConfigHub.
- **The cub-scout cross-check reads the right cluster**, and only where the
  layers put things.
- **`e2e/run.sh`** runs the whole journey on a kind cluster of its own and
  compares every UID at each move.

## 0.1.0, 2026-09-30

The first release. A Flux fleet onboards, hands over and reports back:

- **Plan and onboard.** `cub flux plan` reads a fleet repository and infers each
  layer's shared base from what every cluster's overlay builds on.
  `cub flux apply` writes `apply.sh`, which fills ConfigHub while Flux carries
  on reading Git, and `cleanup.sh`, which takes it back out.
- **A root per cluster.** Each cluster gets one root `Kustomization` reading
  its layers Space, which holds one Unit per layer. `handover.sh` puts it on a
  cluster running the layers from Git — including a fleet bootstrapped from
  Git, where it pauses for the commit that removes the layers — and `join.sh`
  on a new cluster. Each checks the layers arrive at the checked release and
  prints the way back.
- **Status back.** `cub flux status` writes what each layer applied, by exact
  release digest, as `confighub.com/live-status`; `--require Healthy` makes
  each later stage wait for it.
- **New clusters proposed.** `cub flux watch` proposes each cluster added to the
  fleet repository and releases it once a person approves.
- **Evidence.** `cub flux check --fields --record` records each verdict as a
  LiveCheck attestation on the revision it checked.
- **Targets marked for Flux.** Each cluster's Target records its layers Space and
  pull Secret, for a future `cub variant create` that adds a new variant's layer
  there.

Run end to end on Flux v2.8.6 in kind against ConfigHub v0.6.8: see the guide's
"What has and has not been checked".
