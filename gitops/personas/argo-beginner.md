# Argo beginner

## Who they are

An application or DevOps engineer who deploys through Argo CD but did not
design the setup. They know `kubectl`, can read a Deployment and a Kustomize
overlay, and use the Argo CD web UI to check whether an app is Synced and
Healthy. They have not written an ApplicationSet from scratch and have never
used ConfigHub.

## What they bring

- One app, sometimes two or three services, in one or two environments.
- A repo in one of these shapes:
  - an ApplicationSet with a directory or list generator over
    `overlays/dev` and `overlays/prod`;
  - one hand-written Argo `Application` per environment;
  - Helm values files per environment instead of Kustomize overlays.
- One cluster, or one cluster per environment.

## What they want

- To understand what their repo actually deploys, and where.
- To make a small change (an image tag, a replica count, an environment
  variable) in one environment without affecting the other.
- To see the difference between dev and prod before changing either.
- To undo a change quickly when something looks wrong.

## First questions

- "What does this repo deploy, and to which environments?"
- "What is different between dev and prod?"
- "How do I change the replica count in prod only, and see the change before
  it goes out?"
- "Something looks wrong after my change. How do I go back?"
- "I finished the tutorial. How does this apply to the repo I already have?"

## What helped means

1. The answer names the real objects in their repo: the ApplicationSet or
   Applications, the overlays or values files, and the target namespaces.
2. The dev and prod comparison lists the actual fields that differ, and
   nothing else.
3. A proposed change touches only the requested field in the requested
   environment, and shows the exact diff before anything is applied.
4. Rollback is one clear step, and afterwards they can see it worked.
5. Terms such as Space, Unit or variant are explained the first time they
   appear, in Argo words the person already knows.

## Where they get stuck

- Tutorials that create new apps and never connect back to the repo they
  already have.
- Menus and pages that look like the right place to start but are not.
- Not knowing whether Argo, Git or ConfigHub is the source of truth for a
  given field.

## Example

[`../argo/beginner-applicationset`](../argo/beginner-applicationset/README.md)

## See also

- [Walk the whole model once, with Redis](https://confighub.github.io/helm-expt/site/d/docs/user/workshop-redis-intro-guide.html), a beginner tour of ConfigHub in ConfigHub Workshop.
- [GitOps adopter guide](https://confighub.github.io/helm-expt/site/d/docs/user/gitops-adopter-guide.html), for keeping Argo CD in charge while ConfigHub provides the source.
