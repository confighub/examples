# What's new in cub flux

Each release is tagged `cub-flux-v<version>` in confighub/examples and installs
with `cub plugin install confighub/examples@cub-flux-v<version> --name flux`.

## 0.5.1, unreleased

- **A re-run keeps the pull token, and revokes the ones it replaced.** 0.5.0's
  `handover.sh` and `join.sh` issued a new token each run and left the earlier
  ones valid. They now record the token's id on the pull Secret, as the
  annotation `confighub.com/token-jti`, and keep the token while ConfigHub
  lists it as live. Otherwise they issue one and, once the Secret holds it,
  revoke that cluster's other tokens, the ones with the description
  `Flux, cluster <cluster>`. Other clusters' tokens are left alone.
- **Credentials are no longer copied into an annotation, or onto a command
  line.** The pull Secret and fluxbot's Secret were written with
  `kubectl apply`, which keeps a copy of what it applies, token or private key
  included, in the Secret's `kubectl.kubernetes.io/last-applied-configuration`
  annotation. Both are now written with `kubectl create`, or `kubectl replace`
  when they exist, which also drops a copy 0.5.0 left. The pull Secret is the
  same `kubernetes.io/dockerconfigjson` Secret, built from a file descriptor
  rather than `--docker-password`, so the token is on no command line.
- **fluxbot keeps a key ConfigHub lists, and replaces one it does not.**
  `fluxbot.sh` records the key's id on the Secret, as `confighub.com/key-id`.
  A run keeps the key while ConfigHub lists that id. Otherwise, as after the
  service account was deleted and made again, or for a Secret 0.5.0 wrote,
  it adds a key and removes fluxbot's earlier keys on that cluster. The pod
  restarts, because what it was made from changed.
- Checked on 2026-10-26 on a kind cluster against ConfigHub v0.9.5, from
  Secrets written as 0.5.0 writes them:
  - the first run left each Secret with only its own annotation, and revoked
    the two earlier tokens and the earlier key, but not another cluster's
    token;
  - the pull Secret matched what `kubectl create secret docker-registry`
    writes;
  - a second run kept the token and the key;
  - after the service account was made again, the run replaced both.

## 0.5.0, 2026-10-26

Built with ConfigHub SDK v0.9.4.

- **A service account replaces the server worker.** ConfigHub is removing
  workers. `apply.sh` creates the service account `<prefix>-delivery`, with no
  organization role, and grants it `View` and `ViewChildren` on each Target,
  also on a Target an earlier `apply.sh` made. `handover.sh` and `join.sh`
  issue a token of it with `cub serviceaccount token create` and write the pull
  Secret with the username `token` and the token as password. `cleanup.sh`
  deletes the service account, which revokes its tokens and keys.
- **fluxbot signs in with a key.** `fluxbot.sh` grants the service account
  `EditChildren` on the cluster's Target, adds a key of it, and puts the
  private half in fluxbot's Secret as `CONFIGHUB_AUTH_PRIVATE_KEY`. Run again,
  it keeps the key the Secret holds.
- **`cub flux status --as-service-account`** replaces `--as-worker`. It signs
  in to `CONFIGHUB_URL` as the service account whose private key
  `CONFIGHUB_AUTH_PRIVATE_KEY` holds, signs in again whenever ConfigHub refuses
  the session, and stops at once if it cannot sign in at the start.
  `CONFIGHUB_ASSERTION_AUDIENCE` is needed only for a server that sets a
  non-default audience.

A fleet onboarded with an earlier version moves over by running `apply.sh`
again, pointing each cluster's pull Secret at a token
(`cub serviceaccount token create <prefix>-delivery`, then
`kubectl -n flux-system create secret docker-registry confighub-<prefix>-targets
--docker-server=<gateway> --docker-username=token --docker-password=<token>
--dry-run=client -o yaml | kubectl replace -f -`; `replace`, not `apply`, so
no copy of the token is kept in an annotation), and running `fluxbot.sh`
again where it runs. After that nothing uses the worker.

## 0.4.1, 2026-10-25

- **The first release works on ConfigHub v0.9.** `apply.sh` stopped at the
  release step with HTTP 422, "requires approval, but no Revision was found to
  evaluate it on". `cub variant create` already gives each variant the base's
  head, so the change order made afterwards passes over every Unit of the base
  as AlreadyTaken and carries no Revision to approve. When the change order
  skipped every Unit of the base that way, `apply.sh` now publishes what the
  variant holds, which is what its own source renders and what the cluster runs
  today. Any other refusal still stops the script, so a change that carries
  Revisions still waits for its approval. A re-run reports the variant as
  released rather than failing. Run on hub.confighub.com with the beginner
  Flux fleet at confighub/examples 7f1b8f2f: a fresh run and a re-run both
  finished, and each variant has one release.

## 0.4.0, 2026-10-09

- **fluxbot: the live-status reporter, on the cluster.** Flux has no reporter
  of its own, so `cub flux status --watch` had to be kept running somewhere
  for the Healthy gate to open. `cub flux apply` now also writes `fluxbot.sh`,
  which runs that reporter as one pod in the Flux namespace, signed in as the
  Targets' worker, as argobot is for Argo CD. It grants that worker
  `EditChildren` on the cluster's Target, and adds a Secret, a ServiceAccount
  that may read `Kustomization`s and `OCIRepository`s in that namespace, and a
  Deployment. No image is built for it: the pod fetches this release's binary
  from GitHub, checks it against the release's checksum, and runs it in the
  Kubernetes project's `kubectl` image. Run live: a change released to dev
  reached the next stage 87 seconds after its publish with no command running.
- **`cub flux status --discover`** reports every layer on the cluster that
  reads a ConfigHub Space of the fleet, read from the cluster each pass, with
  no fleet repository. It is what fluxbot runs, and works by hand too.
- **`cub flux status --as-worker`** signs in to `CONFIGHUB_URL` as the worker
  `CONFIGHUB_WORKER_ID` and `CONFIGHUB_WORKER_SECRET` name, and stops at once if
  it cannot. Without the flag those variables are not read. `--ready-file`
  writes a file once it has signed in, which the pod's readiness probe reads,
  so a reporter that cannot sign in never replaces one that can.
- **Every request to ConfigHub has a time limit**, so a stalled connection is
  an error rather than a silence.

## 0.3.2, 2026-10-09

- **A recorded check judges health.** `cub flux check --fields --record`
  recorded a Pass whenever the objects and fields matched, even on a layer
  Flux reported failed. Now a layer that is stalled, or not ready for a reason
  of its own, is a rejection that says so, and one still reconciling, waiting
  on a dependency or suspended records nothing, since it is neither yet. A
  Pass names the health it rests on, and that is `Unknown`, not Healthy, where
  neither `spec.wait` nor a health check covers the layer's workloads: ready
  then means applied. With `--record` the command now fails unless every check
  recorded a Pass, and `--json` carries the verdict. A check without
  `--record` is unchanged: it still answers whether a handover would change the
  cluster.
- **A failed layer is reported as failed.** After a pass that fails, Flux
  leaves the layer not ready and also reconciling, since it will retry.
  `cub flux status` 0.3.1 read that as a pass still under way, and at a
  release already applied as nothing new, so a layer whose health check had
  started failing kept its passing reading. It is now recorded as failed. A
  layer that is only waiting on a dependency is not a failure.
- **The gateway address is taken with or without `oci://`** by `apply.sh`,
  `handover.sh` and `join.sh`, as `cub argo`'s scripts take it.
- **Sign-in advice fits a server with no identity provider.** The scripts said
  to run `cub auth login`; they now also give the form that signs in by key.

## 0.3.1, 2026-10-09

Needs ConfigHub v0.8.2 or newer for live status. Built with ConfigHub SDK
v0.8.10.

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
