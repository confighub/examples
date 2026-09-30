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
| `watch` | proposes each cluster added to the fleet repository, and releases it once a person approves | no, it runs `apply.sh`; `join.sh` stays yours |
| `cleanup.sh` | takes what `apply.sh` made back out of ConfigHub, layers Spaces first | no |
| `check` | compares what each layer applied with the published release; `--record` keeps the verdict as a `LiveCheck` attestation | reads it; changes nothing, except that `--record` writes to ConfigHub |
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

For consumers that render a plugin preview, `--format preview-json` emits the
shared `confighub.com/plugin-preview/v1` envelope from the same plan. It
describes the supplied Git plan as inventory and the proposed ConfigHub
Spaces, Targets, and Units; it does not claim live cluster discovery or
delivery. Its `exportedAt` is the export time and changes between runs. Pass
`--exported-at <RFC3339>` to pin it for reproducible output. The existing
`--json` output remains the plan schema and is unchanged.

```bash
./bin/cub-flux plan ../gitops/flux/expert-fleet --format preview-json
./bin/cub-flux plan ../gitops/flux/expert-fleet --format preview-json --exported-at 2026-09-30T00:00:00Z
```

A layer path is read as kustomize-controller reads it. One with a
kustomization is built with kustomize. One without is a plain layer: Flux
generates a kustomization over every `.yaml` and `.yml` below it, recursively,
taking a subdirectory with a kustomization of its own whole. `apply.sh` and
`handover.sh` render it the same way, and the plan names any file there that
is not Kubernetes YAML, which would fail Flux's build.

## What it leaves out

Live exports (`kubectl get kustomizations ...`), OCIRepository and Bucket
sources as layer inputs, `substituteFrom` values, and rendering: it reads
kustomization files but does not run kustomize or Helm.

## Install

```bash
cub plugin install confighub/examples@cub-flux-v0.1.0 --name flux
cub plugin list       # flux should be listed, status ok
cub flux plan ../gitops/flux/expert-fleet
```

Releases are tagged `cub-flux-v<version>` in this repository; upgrade by
naming one: `cub plugin upgrade flux@cub-flux-v<version>`. What each release
changed is in [docs/whats-new.md](docs/whats-new.md).

To build from source instead (needs Go):

```bash
go build -o bin/cub-flux . && cub plugin install ./bin/cub-flux
# after a git pull: go build -o bin/cub-flux . && cub plugin upgrade flux
```

## Prove it on kind

`e2e/run.sh` runs the whole journey for one cluster of `gitops/flux/beginner`
on a kind cluster of its own, with its own kubeconfig: the layers applied from
GitHub, `plan`, `apply.sh`, `handover.sh`, `status`, the way back
`handover.sh` printed, and `cleanup.sh`. It compares every UID at each move and
stops at the first thing that is not as it should be.

```bash
CONFIGHUB_OCI=<gateway host:port the kind cluster reaches> CONFIGHUB_OCI_PLAIN_HTTP=1 \
  bash e2e/run.sh
```

It passed on 2026-09-30 against Flux v2.8.6 and ConfigHub v0.6.8, and its
first run found that the way back left the layers' OCIRepositories reading
ConfigHub; the way back now removes them last. A bootstrapped fleet's handover
pauses for a commit to the repository `flux-system` reads, which a rig reading
GitHub cannot make: [docs/runs/2026-09-30-bootstrapped-handover.md](docs/runs/2026-09-30-bootstrapped-handover.md)
is that run.

## Develop

```bash
make test      # unit tests, the golden plan, and two break-it cases
make golden    # rewrite the golden plan after an intended change
```
