# cub flux

`cub flux` reads a Flux fleet repository as it is and shows the fleet
ConfigHub would govern. In Flux each cluster already has its own directory,
so `plan` works backwards: for each layer (a Flux Kustomization such as
`infrastructure` or `apps`) it infers the shared base from what every
cluster's overlay builds on, and lists exactly what each cluster's overlay
changes. Those changes are the variant's departures.

The first two commands change nothing. The scripts `apply` writes are the ones
that move a cluster:

| Command | What it does | Touches a cluster? |
|---|---|---|
| `plan` | shows the fleet ConfigHub would govern | no, and no account either |
| `apply --out` | writes the workflow files and `apply.sh`, `handover.sh`, `join.sh` and `cleanup.sh` | no, it runs nothing |
| `apply.sh` | fills ConfigHub with a parallel copy nothing reads, and one layers Space per cluster (needs `CONFIGHUB_OCI`) | no |
| `handover.sh` | on a cluster running the layers from Git, puts one root on the cluster, which takes each layer over | yes, this is the step that moves it |
| `join.sh` | on a new cluster with Flux and none of the layers, puts the root there and waits for the layers to arrive | yes, it adds to an empty cluster |
| `cleanup.sh` | takes what `apply.sh` made back out of ConfigHub, layers Spaces first | no |
| `check` | compares what each layer applied with the published release | reads it, changes nothing |
| `status` | reports what Flux applied as ConfigHub live status, which the Healthy gate reads | reads it; writes only to ConfigHub |

`flux-system` is never repointed: it reconciles the Flux controllers
themselves, so it stays on Git as the recovery path. It also holds the root and
the gateway credential. The shape is the one `cub cluster up` makes for Argo CD:
a Space per cluster, and one root that reads it.

**Start with the guide: [Onboard your Flux fleet](docs/onboard-your-flux-fleet.md).**

## Try it on the expert example

```bash
make example
# the same as:
go build -o bin/cub-flux .
bin/cub-flux plan ../gitops/flux/expert-fleet
```

The committed output is [testdata/expert-fleet.plan.txt](testdata/expert-fleet.plan.txt).
It answers the questions the
[example's README](../gitops/flux/expert-fleet/README.md) asks of any tool
that claims to read it:

| Question | Where the plan answers it |
|---|---|
| Which clusters, in what order | `dev-1`, `staging-1`, `prod-1`, each its own stage |
| What reconciles first | `infrastructure -> apps, tenants -> image-automation`, from `dependsOn` |
| What each cluster changes | Per variant: namespace, labels, image tags, each patch op, postBuild values, and Flux fields that differ |
| What is in flight | `nginx: 1.27.3-alpine on dev-1; 1.27.2-alpine on staging-1; 1.26.3-alpine on prod-1` |
| How a change reaches production | `fleet-repo` and `team-checkout` read `production` on prod-1 and `main` elsewhere |
| What writes to Git by itself | `ImageUpdateAutomation apptique-dev`, into `apps/dev`, on dev-1 only |
| What is not in Git | `${cluster_name}` and `${environment}`, and what the `edge-router` chart renders |
| Tenants and bootstrap | `team-checkout` runs as its own ServiceAccount; `clusters/*/flux-system` stays outside |

It also catches a cluster that stops supplying a substituted value (Flux
would apply `${environment}` literally) and a layer path that does not exist.

## Inputs

A fleet repository directory with one directory per cluster under
`clusters/` (`--clusters` to change it). Flux paths are resolved against the
checkout found by walking up to `.git`, or `--repo-root`. Clusters roll out
dev, staging, prod, then the rest by name; `--stages` sets the order.
`--json` prints the plan as JSON.

## What it leaves out

Live exports (`kubectl get kustomizations ...`), OCIRepository and Bucket
sources as layer inputs, `substituteFrom` values, and rendering: it reads
kustomization files but does not run kustomize or Helm.

## Use it as a cub plugin

The plugin is built from this directory, since it does not live in a
repository of its own. It needs Go.

```bash
make install-plugin   # builds into $CUB_CONFIG/plugins/cub-flux
cub plugin list       # cub-flux should be listed, status ok
cub flux plan ../gitops/flux/expert-fleet
```

## Develop

```bash
make test      # unit tests, the golden plan, and two break-it cases
make golden    # rewrite the golden plan after an intended change
```
