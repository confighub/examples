# GitOps examples

This is the catalog of canonical Argo CD and Flux examples in this repo. If
you run Argo CD or Flux, and you want an example that looks like your setup,
start here.

Each example is a small, realistic repo shape: the kind of Argo or Flux
layout a real team would actually have. Each one renders offline (no cluster
needed) and can be uploaded into ConfigHub so you can inspect, diff, and
change it safely before anything touches a live cluster.

## Catalog

| Shape | Tool | Level | Status | Example |
|---|---|---|---|---|
| One app, ApplicationSet | Argo CD | Beginner | Ready | [`argo/beginner-applicationset`](./argo/beginner-applicationset/README.md) |
| One app, Flux Kustomizations with a clusters, infrastructure, apps split | Flux | Beginner | Ready | [`flux/beginner`](./flux/beginner/README.md) |
| CI writes image tags to a separate GitOps repo, promotion by pull request | Argo CD | Intermediate | Planned | not yet added |
| App-of-apps plus ApplicationSets, cluster labels, staged rollout, sync windows | Argo CD | Expert | Planned | not yet added |
| Bootstrap plus clusters plus apps, namespace per team, branch promotion, post-build substitution | Flux | Expert | Planned | not yet added |
| Several teams on one cluster | Flux | Expert | Planned | not yet added |
| Drift, failed sync, and bad-commit states | Argo CD and Flux | Expert | Planned | not yet added |

"Ready" means the example is fully built: it has a README, an AI guide, a
machine-readable contract, and scripts that render and can upload into
ConfigHub. "Planned" rows are tracked and will land as their own pull
requests, beginner examples first.

## Personas

Each example is written for one persona. See [`personas/`](./personas/README.md)
for the Argo and Flux beginner and expert descriptions.

## Which example is like my repo?

- **I have one app in one or two environments, and I use Argo CD's
  ApplicationSet to manage them.** Start with
  [`argo/beginner-applicationset`](./argo/beginner-applicationset/README.md).
- **I have one app in one or two environments, and I use Flux
  Kustomizations, with a `clusters/`, `infrastructure/`, `apps/` split.**
  Start with [`flux/beginner`](./flux/beginner/README.md).
- **I manage many apps across many clusters with an app-of-apps pattern, or
  I need staged rollout and sync windows.** No expert Argo example exists
  yet; it is planned (see the table above).
- **I run several teams on one Flux-managed cluster, or use branch-based
  promotion.** No expert Flux example exists yet; it is planned (see the
  table above).
- **I want to see what a broken GitOps state looks like (drift, a failed
  sync, a bad commit) and how it gets caught.** No broken-state example
  exists yet; it is planned (see the table above).

If none of these match, the two beginner examples are still the fastest way
to see how this repo's ConfigHub upload and verification flow works: read
[`argo/beginner-applicationset`](./argo/beginner-applicationset/README.md)
or [`flux/beginner`](./flux/beginner/README.md), then adapt the shape to
your own repo.

## A note on the older import examples

[`incubator/gitops-import-argo`](../incubator/gitops-import-argo/README.md)
and [`incubator/gitops-import-flux`](../incubator/gitops-import-flux/README.md)
use an import route that is being retired. If you are looking for a
canonical example to run or to point an AI assistant at, use this catalog
instead.

## Contract standard

Every example in this catalog follows
[`../EXAMPLE_CONTRACT_STANDARD.md`](../EXAMPLE_CONTRACT_STANDARD.md) and the
[AI guide standard](../incubator/ai-guide-standard.md): a `README.md`, an
`AI_START_HERE.md`, a `contracts.md`, and a `setup.sh` that supports
`--explain` and `--explain-json` without mutating anything.
