# cub argo

`cub argo` reads an Argo CD estate as it is and shows the fleet ConfigHub
would govern. It follows the shape of
[`cub sveltos`](https://github.com/confighub/sveltos-confighub): one base per
ApplicationSet, one variant per Application it generates, each addressed to
one cluster, and a Target per cluster. The AppProjects and the tree's shape
stay; `root` and each app of apps are repointed at ConfigHub, not recreated.

The commands and scripts, in order; the first two change nothing:

| Command | What it does | Touches a cluster? |
|---|---|---|
| `plan` | shows the estate ConfigHub would govern | no, and no account either |
| `apply --out` | writes the files and the scripts below | no, it runs nothing |
| `apply.sh` | fills ConfigHub with a parallel copy nothing reads | no |
| `check` | compares what Argo owns on the cluster with what ConfigHub holds; `handover.sh` runs it before it moves anything | no (`--record` writes a LiveCheck to ConfigHub) |
| `handover.sh` | repoints `root` at ConfigHub, prints the reviewed edit for each app of apps, and checks every Application's release against the cluster before any workload's source moves | yes, this is the step that moves it |
| `move-applications.sh` | makes each Application that an ApplicationSet generated a Unit reading its own Space, one stage at a time | yes, through the parent that syncs it |
| `argobot.sh` | runs argobot beside Argo CD: live status goes back to each Space (and releases land at once, with an argobot that has confighub/argobot#14) | yes, it installs argobot |
| `status` | without argobot: writes what each handed-over Application synced into its Space as ConfigHub live status, once or with `--watch` | only with `--hard-refresh`, which asks Argo to read a new release |
| `cleanup.sh` | the way back out of ConfigHub, once each source is back on Git | no |

**Start with the guide: [Onboard your Argo CD estate](docs/onboard-your-argo-estate.md).**

## Install

```bash
cub plugin install confighub/examples@cub-argo-v0.3.0 --name argo
cub plugin list                  # argo should be listed, status ok
cub argo plan <the repository your Argo CD syncs, or a kubectl export>
```

`plan` needs no ConfigHub account and changes nothing, so it is the safe first
command on your own estate. Already on an earlier release?

```bash
cub plugin upgrade argo@cub-argo-v0.3.0
```

Releases are tagged `cub-argo-v<version>` in this repository, and what each one
changed is in [docs/whats-new.md](docs/whats-new.md).

To build from source instead (needs Go):

```bash
go build -o bin/cub-argo . && cub plugin install ./bin/cub-argo
# after a git pull: go build -o bin/cub-argo . && cub plugin upgrade argo
```

## Try it on the expert example, from a checkout

```bash
make example
# the same as:
go build -o bin/cub-argo .
bin/cub-argo plan ../gitops/argo/expert-app-of-apps \
  --stage-label rollout-phase --stages canary,secondary,primary
```

The committed output is [testdata/expert-app-of-apps.plan.txt](testdata/expert-app-of-apps.plan.txt).
It answers the questions the
[example's README](../gitops/argo/expert-app-of-apps/README.md) asks of any
tool that claims to read it:

| Question | Where the plan answers it |
|---|---|
| Roots, generators, leaves | The control tree: `root`, applied by hand, grows the projects, the platform ApplicationSet and the `storefront` app of apps, which grows two ApplicationSets |
| Which clusters, which namespaces | One variant per Application, with its cluster, Target and namespace |
| What is in flight | `in flight  nginx: 1.27-alpine on dev-1, staging-1; 1.26-alpine on prod-1` |
| The Helm-in-Kustomize wrinkle | A note on `checkout-cache`: Argo CD needs `--enable-helm`, and chart resources ignore the overlay's namespace |
| What the prod sync window means | It covers `prod-1-apptique` and `prod-1-checkout-cache`, so a published release waits for the window |
| What a change reaches | A change to a base reaches every variant in stage order; a departure is one variant's |

It also catches the README's first "break it on purpose" edit. Change
`env: prod` to `env: production` in `clusters/prod-1.yaml`, and `plan` exits
non-zero naming the three overlays that do not exist. A cluster missing a
label a template reads is caught too: Argo CD renders it as `<no value>` and
syncs it anyway.

## What a handover does

Each layer is **repointed**, not orphaned. An app of apps does not own its
children through `ownerReferences`; it owns them by syncing a directory and
pruning what is not in it. So there is nothing to sever, and repointing the
parent leaves the tree, every name and every Argo tracking ID intact.

The plan orders it top down, because a parent left syncing an empty source
prunes its children. For the expert example that is: fix `sourceRepos` on both
AppProjects, check Argo is v3.1 or newer, publish `argo-root-children` and
repoint `root` by hand, publish `argo-storefront-children` and repoint the
`storefront` Unit under approval, then retire each ApplicationSet and move the
Applications it generated onto Units, a stage at a time. Every Application is
edited in place and keeps its UID: see [What repointing
means](docs/onboard-your-argo-estate.md#what-repointing-means). Nothing is
deleted, which matters because `root`, `storefront` and every generated
Application carry `resources-finalizer.argocd.argoproj.io`.

Orphaning with `--cascade=orphan` remains the fallback for an ApplicationSet
applied by hand, where no parent can repoint its template under review.

## Inputs

- A repository directory, as above. Source paths are checked against the
  checkout found by walking up to `.git`, or `--repo-root`.
- A live export, which also names the generated Applications a handover would
  keep:

  ```bash
  { kubectl get applications,applicationsets,appprojects -n argocd -o yaml; echo '---'; \
    kubectl get secrets -n argocd -l argocd.argoproj.io/secret-type=cluster -o yaml; } \
    | bin/cub-argo plan -
  ```

  Only cluster names, servers and labels are read from cluster Secrets, never
  their credentials.

`--json` prints the plan as JSON, for scripts and agents.

## What it reads, and what it leaves out

Reads cluster, list and matrix (two generators) generators; Go templates
(`goTemplate`) and `{{param}}` templates; label selectors with `In`, `NotIn`,
`Exists` and `DoesNotExist`; sync waves; sync windows; overlay image tags.

Reads a source path as Argo does: a kustomization is built with kustomize, and
a plain directory of manifests is read file by file, recursively with
`directory.recurse`. Argo CD's own cluster, `in-cluster`, needs no cluster
Secret.

Leaves out, and says so: git, SCM provider, pull request, merge and plugin
generators; templates that use Sprig functions; multi-source Applications'
paths; Helm chart sources; plain directories read with include, exclude or
jsonnet. It does not render Kustomize or Helm; for that, use the
[ConfigHub Workshop](https://confighub.github.io/helm-expt/) or `cub gen`.

## After the handover: live status

argobot.sh is the first route: it reports live status and lands each release.
Where argobot is not running, keep this running beside the cluster instead:

```bash
cub argo status ../gitops/argo/expert-app-of-apps --stage-label rollout-phase \
  --stages canary,secondary,primary --kube-context <argo cluster> --watch --hard-refresh
```

It writes each Space's `confighub.com/live-status`, which ConfigHub's Healthy
gate, its change orders and its UI read. A reading says `Synced` only when Argo
synced the newest published release of that Space, at that release's digest.
`--hard-refresh` asks Argo, once per release, to read a release it has cached
past; without it, a newly approved release sits unread on the gateway.
`--dry-run` shows what would be written; `--json` prints it for scripts.

## Prove it on kind

`e2e/run.sh` runs the whole journey on a kind cluster of its own, with its own
kubeconfig: an estate syncing from GitHub, a live export, `plan`, `apply.sh`,
`handover.sh`, `status`, the way back `handover.sh` printed, and `cleanup.sh`.
It compares every UID at each move and stops at the first thing that is not as
it should be.

```bash
CONFIGHUB_OCI=<gateway host:port the kind cluster reaches> CONFIGHUB_OCI_PLAIN_HTTP=1 \
  bash e2e/run.sh app-of-apps     # or: by-hand
```

`app-of-apps` is `beginner-app-of-apps`, a root whose children sync plain
directories. `by-hand` is `intermediate-ci-to-gitops`, Applications with no
parent, whose image does not pull, so `status` has to report `Progressing` and
keep the Healthy gate shut. Both passed on 2026-09-30 against Argo CD v3.5.3
and ConfigHub v0.6.8, and the script's own first runs found three bugs in
itself, none in the plugin.

## Develop

```bash
make test      # unit tests, the golden plan, and two break-it cases
make golden    # rewrite the golden plan after an intended change
```

What has been run live, and what it found, is in the guide's "What the Argo
handover has been through" and in [docs/runs](docs/runs).

Where it goes next: move the planning core it shares with `cub sveltos` into
one library.
