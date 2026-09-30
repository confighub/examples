# cub argo

`cub argo` reads an Argo CD estate as it is and shows the fleet ConfigHub
would govern. It follows the shape of
[`cub sveltos`](https://github.com/confighub/sveltos-confighub): one base per
ApplicationSet, one variant per Application it generates, each addressed to
one cluster, and a Target per cluster. The root Application, its app of apps
and the AppProjects stay as they are, as the management record.

Three commands, and the first two change nothing:

| Command | What it does | Touches a cluster? |
|---|---|---|
| `plan` | shows the estate ConfigHub would govern | no, and no account either |
| `apply --out` | writes the files, `apply.sh` and `handover.sh` | no, it runs nothing |
| `apply.sh` | fills ConfigHub with a parallel copy nothing reads | no |
| `handover.sh` | repoints each layer's source at ConfigHub, top down | yes, this is the step that moves it |

**Start with the guide: [Onboard your Argo CD estate](docs/onboard-your-argo-estate.md).**

## Try it on the expert example

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
`storefront` Unit under approval, then point each ApplicationSet's template at
its clusters' Targets. Nothing is deleted, which matters because `root` and
`storefront` carry `resources-finalizer.argocd.argoproj.io`.

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

Leaves out, and says so: git, SCM provider, pull request, merge and plugin
generators; templates that use Sprig functions; multi-source Applications'
paths. It does not render Kustomize or Helm; for that, use the
[ConfigHub Workshop](https://confighub.github.io/helm-expt/) or `cub gen`.

## Use it as a cub plugin

The plugin is built from this directory, since it does not live in a
repository of its own. It needs Go.

```bash
go build -o bin/cub-argo . && cub plugin install ./bin/cub-argo
cub plugin list       # argo should be listed, status ok
# after a git pull: go build -o bin/cub-argo . && cub plugin upgrade argo
cub argo plan ../gitops/argo/expert-app-of-apps --stage-label rollout-phase --stages canary,secondary,primary
```

## Develop

```bash
make test      # unit tests, the golden plan, and two break-it cases
make golden    # rewrite the golden plan after an intended change
```

Where it goes next: rehearse `handover.sh` on kind and publish the
measurement, the way `cub sveltos` did, and move the planning core it shares
with `cub sveltos` into one library.
