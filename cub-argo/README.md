# cub argo

`cub argo` reads an Argo CD estate as it is and shows the fleet ConfigHub
would govern. It follows the shape of
[`cub sveltos`](https://github.com/confighub/sveltos-confighub): one base per
ApplicationSet, one variant per Application it generates, each addressed to
one cluster, and a Target per cluster. The root Application, its app of apps
and the AppProjects stay as they are, as the management record.

This is a scaffold. It ships `plan`, which is offline and changes nothing.
`apply` (write the steps as a script) and `takeover` are next; the plan
already says what takeover would involve.

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

## Inputs

- A repository directory, as above. Source paths are checked against the
  checkout found by walking up to `.git`, or `--repo-root`.
- A live export, which also shows the generated Applications a takeover would
  adopt:

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

```bash
make install-plugin   # builds into $CUB_CONFIG/plugins/argo/main
cub argo plan ../gitops/argo/expert-app-of-apps --stage-label rollout-phase --stages canary,secondary,primary
```

## Develop

```bash
make test      # unit tests, the golden plan, and two break-it cases
make golden    # rewrite the golden plan after an intended change
```

Where it goes next is in the design brief: move the planning core it shares
with `cub sveltos` into one library, reuse `cub gen`'s ApplicationSet
detection, then add `apply --out` and `takeover`.
