# What's new in cub flux

Each release is tagged `cub-flux-v<version>` in confighub/examples and installs
with `cub plugin install confighub/examples@cub-flux-v<version> --name flux`.

## 0.3.1, unreleased

Needs ConfigHub v0.8.2 or newer for live status.

- **Live status is recorded where ConfigHub now reads it.** From ConfigHub
  v0.8.2, live status lives on the Release, and the Healthy gate reads the
  newest published release of a Space and nothing else. `cub flux status` wrote
  the Space annotation `confighub.com/live-status`, which a current server does
  not read, so a fleet planned with `--require Healthy` never passed the gate.
  It now records each reading on the release whose digest Flux reports, in
  ConfigHub's normalized words.
- **A failing release is reported on the release that failed.** When Flux is
  applying a release, or stalled on one, the reading goes on that release
  rather than the one applied before it, so the gate says that release is not
  synced, instead of that it has no live status yet.
- **A reading that is no longer true is withdrawn.** If the newest release
  holds a passing reading this reporter wrote and Flux now reports another
  release, or none, that reading is replaced, so the gate does not go on
  passing on it.
- **A routine reconcile does not close the gate.** Flux marks a layer
  Reconciling at the start of every pass, including the one it makes each
  interval over a release it already applied. That pass is no longer reported
  as out of sync; what is recorded stands until it finishes.
- **Another reporter's reading is left alone** while it is fresh, or says the
  same.

## 0.3.0, 2026-10-02

Needs `cub` and ConfigHub v0.7.0 or newer.

- **ConfigHub is asked through the SDK.** `cub flux check`, `check --record`,
  `status` and `watch` read releases, revisions, Units, Targets and Spaces, and
  write live status and attestations, through the ConfigHub SDK (v0.8.0) in the
  plugin's own process. They no longer run `cub` and read what it prints, so
  they do not break when its output changes, and `check` and `status` do not
  need `cub` on the PATH. They use the login `cub` passes a plugin, or the
  active context. The scripts `cub flux apply` writes still call `cub`, since
  they are for you to read and run; `watch` runs one of them, so it still
  needs `cub`.
- **Targets without a worker.** `apply.sh` creates each Target the way `cub`
  v0.7.0 does, with no worker, provider or parameters. `cub flux` 0.2.0 fails
  against `cub` v0.7.0 at that step.
- **The guide says what repointing means**, with a diff of the one edit and
  what happens to each kind of object.

## 0.2.0, 2026-09-30

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
