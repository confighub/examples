# Flux beginner

## Who they are

A DevOps or application engineer who looks after a Flux setup that someone
else built, often a contractor, a previous team member or a starter template.
They know `kubectl`, can read Kustomize patches and a HelmRelease, and run
`flux get kustomizations` when something is stuck. They have not designed a
Flux repo and have never used ConfigHub.

## What they bring

- One app, or a handful of services, in one or two environments.
- A repo in one of these shapes:
  - the common split of `clusters/<env>/`, `infrastructure/` and `apps/`,
    with one Flux Kustomization per layer;
  - a single GitRepository feeding one Kustomization per service;
  - a HelmRelease per environment with inline values.
- Little or no documentation of why it is laid out this way.

## What they want

- To understand what the setup does before touching it.
- To know which files are safe to change and which would break something.
- To make one small change in one environment.
- To see whether Git and the cluster agree.

## First questions

- "What does this Flux repo deploy, and in what order?"
- "Which Kustomization or HelmRelease owns this Deployment?"
- "Is what is running the same as what is in Git?"
- "How do I change a value in dev only, and preview it first?"

## What helped means

1. The answer maps each Flux Kustomization, HelmRelease and source to what it
   deploys, including dependency order.
2. Ownership of a given resource is traced to one file and one Flux object.
3. Any difference between Git and the cluster is shown as a short list, with
   no difference invented.
4. A proposed change is limited to the requested environment and shows the
   exact diff before anything is applied.
5. ConfigHub terms are explained in Flux words the person already knows.

## Where they get stuck

- Several layers of Kustomizations with no README.
- Values spread across overlays, HelmRelease values and post-build
  substitution.
- Not knowing whether Flux will overwrite a change made anywhere else.

## Example

[`../flux/beginner`](../flux/beginner/README.md)
