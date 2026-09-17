# Flux expert

## Who they are

A platform engineer who owns a Flux fleet repo used by several teams. They
know Kustomizations, HelmReleases, GitRepository and OCIRepository sources,
dependency ordering, post-build substitution and multi-tenancy with
namespaces and service accounts. They automate updates with tools such as
Renovate and validate changes before merge. They are comfortable writing
glue code.

## What they bring

A running fleet in one of these common shapes:

| Shape | Looks like |
|---|---|
| Fleet repo with bootstrap, clusters and apps | `bootstrap/`, `clusters/<name>/`, `apps/<namespace>/<app>/`, shared infrastructure layers |
| Multi-tenant cluster | Platform team owns infrastructure; each team owns a namespace and its own repo or folder |
| Branch or directory promotion | `main` feeds pre-production, a `production` branch or directory feeds live |
| OCI delivery | Rendered manifests pushed as OCI artifacts and pulled by OCIRepository |

Usually several clusters (cloud and on-premises), dozens of apps, and many
repositories kept current by automation.

## What they want

- One view of what each cluster and team runs, and which source it came
  from.
- Promotion between environments that is reviewed and traceable, not a
  manual branch rebase or copy of values.
- Guardrails so a change from one team cannot break another team or the
  platform layer.
- A gradual way in that keeps Flux reconciling and does not require
  rewriting the repo.

## First questions

- "Can ConfigHub map my fleet repo, clusters and tenants without changing
  anything?"
- "Which parts would ConfigHub own, and which stay in Git with Flux?"
- "How would promoting a change from pre-production to live work here, and
  who approves it?"
- "What happens to dependency ordering, post-build substitution and image
  automation?"
- "Can Flux pull what ConfigHub publishes as OCI?"

## What helped means

1. The answer identifies the repo shape, tenants, clusters and sources
   correctly.
2. Ownership is explained before any change is proposed, including which
   team owns each layer.
3. The first step offered is read-only and changes nothing in the cluster or
   the repos.
4. A promotion shows the exact diff, the environments and clusters it
   reaches, the approval it needs, and how to undo it.
5. Tenant boundaries are respected: a change scoped to one team never
   touches another team or the platform layer without saying so.
6. Limits are stated plainly and match the current tools.

## Where they get stuck

- Tools that assume a single-repo, single-cluster layout.
- Values that only exist after post-build substitution or Helm rendering.
- Automated update pull requests arriving faster than anyone can review
  their combined effect.

## Scenarios

- **Map a fleet:** clusters, tenants, sources and apps, read-only.
- **Reviewed promotion:** one app moves from pre-production to live with an
  approval and a rollback path.
- **Tenant guardrail:** a change from one team is stopped before it reaches
  shared infrastructure.

## Example

[`flux/expert-fleet`](../flux/expert-fleet/README.md): three clusters, four
layers ordered with `dependsOn`, an infrastructure and apps split, a
HelmRelease from a chart source, image automation on dev only, one tenant
with its own service account, and a dev to staging to production promotion
path written into the layout. A several-teams-on-one-cluster example is
still planned.
