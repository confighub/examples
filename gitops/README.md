# GitOps examples

This is the index of canonical Argo CD and Flux examples in this repo. If
you run Argo CD or Flux, and you want an example that looks like your setup,
start here.

Each example is a small, realistic repo shape: the kind of Argo or Flux
layout a real team would actually have. Each one renders offline (no cluster
needed) and can be uploaded into ConfigHub so you can inspect, diff, and
change it safely before anything touches a live cluster.

## Index

| Shape | Tool | Level | Status | Example |
|---|---|---|---|---|
| One app, ApplicationSet | Argo CD | Beginner | Ready | [`argo/beginner-applicationset`](./argo/beginner-applicationset/README.md) |
| One app, app of apps: a root Application and one child per environment | Argo CD | Beginner | Ready | [`argo/beginner-app-of-apps`](./argo/beginner-app-of-apps/README.md) |
| One app, Flux Kustomizations with a clusters, infrastructure, apps split | Flux | Beginner | Ready | [`flux/beginner`](./flux/beginner/README.md) |
| CI writes image tags to a separate GitOps repo, promotion by pull request | Argo CD | Intermediate | Planned | not yet added |
| A fleet repo doing a database's job: filename keys, silent layers, find-and-replace promotion, reach nobody can see | Argo CD | Intermediate | Ready (read-only; ConfigHub upload planned) | [`argo/intermediate-git-as-database`](./argo/intermediate-git-as-database/README.md) |
| App-of-apps plus ApplicationSets, cluster labels, staged rollout, sync windows | Argo CD | Expert | Ready | [`argo/expert-app-of-apps`](./argo/expert-app-of-apps/README.md) |
| Bootstrap plus clusters plus apps, namespace per team, branch promotion, post-build substitution | Flux | Expert | Ready | [`flux/expert-fleet`](./flux/expert-fleet/README.md) |
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
- **I have one app in one or two environments, and a root Argo CD
  Application creates one child Application per environment.** Start with
  [`argo/beginner-app-of-apps`](./argo/beginner-app-of-apps/README.md).
- **I have one app in one or two environments, and I use Flux
  Kustomizations, with a `clusters/`, `infrastructure/`, `apps/` split.**
  Start with [`flux/beginner`](./flux/beginner/README.md).
- **I manage many apps across many clusters with an app-of-apps pattern, or
  I need staged rollout and sync windows.** Start with
  [`argo/expert-app-of-apps`](./argo/expert-app-of-apps/README.md): a root
  app, sync waves, a matrix ApplicationSet over three clusters, a production
  sync window, and a child app-of-apps that owns two apps.
- **I run a Flux fleet with layered Kustomizations, a tenant per team, image
  automation, or branch-based promotion.** Start with
  [`flux/expert-fleet`](./flux/expert-fleet/README.md): three clusters, four
  layers with dependency ordering, a HelmRelease from a chart source, image
  automation on dev only, one tenant with its own service account, and a dev
  to staging to production promotion path written into the layout.
- **I run several teams on one Flux-managed cluster and want the
  multi-tenancy shape on its own.** `flux/expert-fleet` includes one tenant;
  a dedicated several-teams example is still planned (see the table above).
- **I want to see what a broken GitOps state looks like (drift, a failed
  sync, a bad commit) and how it gets caught.** No broken-state example
  exists yet; it is planned (see the table above).

If none of these match, the two beginner examples are still the fastest way
to see how this repo's ConfigHub upload and verification flow works: read
[`argo/beginner-applicationset`](./argo/beginner-applicationset/README.md)
or [`flux/beginner`](./flux/beginner/README.md), then adapt the shape to
your own repo.

## ConfigHub Workshop

[ConfigHub Workshop](https://confighub.github.io/helm-expt/) covers the ConfigHub side of the same journey:

- [Walk the whole model once, with Redis](https://confighub.github.io/helm-expt/site/d/docs/user/workshop-redis-intro-guide.html): a beginner tour of the model over one chart.
- [GitOps adopter guide](https://confighub.github.io/helm-expt/site/d/docs/user/gitops-adopter-guide.html): keeping Argo CD or Flux in charge while ConfigHub provides the source.
- [Model and vocabulary](https://confighub.github.io/helm-expt/site/d/docs/user/model-and-vocabulary.html): what Workshop means by catalog, base variant, stack and fleet.

## A note on the older import examples

The older Argo CD and Flux import examples used an import route that is
being retired, and they have been removed. Use this index for a canonical example to run or to point an AI
assistant at.

## Contract standard

Every example in this index follows
[`../EXAMPLE_CONTRACT_STANDARD.md`](../EXAMPLE_CONTRACT_STANDARD.md) and the
[AI guide standard](../docs/ai-guide-standard.md): a `README.md`, an
`AI_START_HERE.md`, a `contracts.md`, and a `setup.sh` that supports
`--explain` and `--explain-json` without mutating anything.
