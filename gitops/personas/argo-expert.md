# Argo expert

## Who they are

A platform engineer on a small team that has run Argo CD in production for a
year or more, with product teams deploying through it. They know
Applications, ApplicationSets and their generators, AppProjects, sync waves,
hooks and sync windows. They read controller logs and the Argo docs before
asking anyone, and will script against an API or SDK when a tool falls short.

## What they bring

A live Argo estate in one of these common shapes:

| Shape | Looks like |
|---|---|
| App-of-apps monorepo | A root Application over `apps/<env>/<service>.yaml`, Kustomize overlays per environment |
| ApplicationSets across clusters | Cluster or matrix generators selecting clusters by label, values per region |
| Helm umbrella per environment | Platform charts (cert-manager, external-secrets, ingress, monitoring) with values files |
| Separate app and GitOps repos | CI writes image tags into the GitOps repo; production changes go through a pull request |

Typically several clusters, a mix of product services and shared platform
add-ons, and more than one team committing.

## What they want

- One view of what is running where across clusters, including the objects
  Argo renders from Helm charts.
- Safer changes that affect many clusters: reviewed diffs, staged rollout
  (for example canary clusters first) and a clear way back.
- Clear ownership of the risky parts: root apps, cluster targeting and shared
  platform values.
- A gradual way in that starts read-only and leaves Argo in charge of
  syncing.

## First questions

- "Can ConfigHub show me what my Argo setup deploys today without changing
  anything?"
- "Which parts would ConfigHub own, and which stay in Git and Argo?"
- "How would a change that touches 30 clusters be reviewed and rolled out in
  stages?"
- "What happens to drift, hooks, sync waves and CRD ordering?"
- "What works today, what needs glue code, and what is a feature request?"

## What helped means

1. The answer identifies their shape correctly, including which objects are
   roots, generators and leaves.
2. Ownership is explained before any change is proposed: what ConfigHub
   holds, what stays in Git, and what Argo keeps doing.
3. The first step offered is read-only and changes nothing in the cluster or
   the repos.
4. A multi-cluster change shows the exact diff, the clusters it reaches, the
   order it rolls out in, and how to stop or undo it.
5. Limits are stated plainly and match the current tools, with no route that
   is known to be unreliable or retired.
6. Hooks, sync waves and CRD ordering are handled or clearly flagged, not
   silently dropped.

## Where they get stuck

- Tutorials and quick starts that build a new setup instead of adopting the
  one they run.
- Import paths that only fit one repo shape.
- Objects Argo renders from charts, and live objects that need cleaning
  before they can be stored as configuration.
- Bootstrap steps that still have to be done by hand, such as applying a
  root app.

## Scenarios

- **Adopt an existing app-of-apps step by step:** read-only inventory first,
  then manage the Argo Application objects themselves, while leaf apps keep
  their current sources.
- **Staged change across a fleet:** one platform value changes on canary
  clusters, is checked, then reaches the rest.

## Example

Planned: an app-of-apps plus ApplicationSets repo over several clusters,
built from `incubator/apptique-argo-app-of-apps`.
