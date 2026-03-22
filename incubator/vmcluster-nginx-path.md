# `cub-vmcluster` To Nginx

This note is the smallest realistic follow-on path after `cub-vmcluster` bootstrap.

The authoritative runnable implementation lives in Jesper's [`cub-vmcluster`](https://github.com/jesperfj/cub-vmcluster) repo, which already proves the real cluster plus nginx path.

This page has a different job. It explains why that path matters in the official `examples/incubator` story and how it connects cluster bootstrap to later ConfigHub deployment examples.

The goal is simple:

- start with a real cluster
- wait for the target to appear
- deploy one simple workload
- verify that the cluster is actually serving traffic

This is not the current front door for ConfigHub. The import-first wedge still comes first. This note is the next bridge into live deployment once the cluster and target story is clear.

## The Mental Model

Think about it in this order:

1. bootstrap the cluster
2. capture the target ref
3. load or create a small nginx workload
4. bind that workload to the target
5. apply it
6. verify it from the cluster and, if available, from the edge

That keeps the story simple.

In other words:

- first you make the cluster usable
- then you point a tiny app at it
- then you prove the app is really there

## Why Nginx Is The Right First Workload

Nginx is a good first live deployment because it keeps the moving parts small.

It is easy to reason about:

- one namespace
- one deployment
- one service
- optionally one ingress

That makes it a good handoff after `vmcluster/bootstrap` and a good baseline before moving into GitOps import, larger app installs, or layered deployment variants.

## What This Page Is For

This page is not trying to recreate Jesper's runnable flow inside this repo.

Use the `cub-vmcluster` repo when the goal is:

- actually boot the cluster
- actually deploy nginx
- verify ingress and TLS on the live system

Use this page when the goal is:

- explain the smallest downstream deployment after target creation
- connect that path to ConfigHub targets and deployment units
- describe the bounded procedure that `cub-proc` would eventually represent

## What The User Cares About

Most users still care about two things:

- the target is real and usable
- the workload is visibly running

At this point the worker is still mostly implementation detail. It matters if the target never appears or apply never happens, but it is not the main thing the user is trying to manage.

## Recommended Flow

The clean path is:

1. run the cluster bootstrap flow
2. wait for the target ref
3. load or create the nginx workload package
4. bind the deploy unit to the new target
5. apply through the direct Kubernetes path
6. verify namespace, deployment, service, and optional ingress

If the cluster has DNS, ingress, and TLS configured, the strongest final proof is a reachable HTTPS URL.

If not, the minimum useful proof is:

- namespace exists
- deployment is available
- service exists

## Evidence Model

This path needs evidence from three places:

- ConfigHub facts
  - target exists
  - workload unit exists
  - workload unit is bound to the target
- cluster facts
  - namespace exists
  - deployment is available
  - service exists
  - ingress exists if requested
- edge facts
  - URL resolves
  - HTTPS responds if TLS is configured

This is the same pattern we have been using elsewhere:

- ConfigHub for intent and binding facts
- cluster inspection for raw runtime facts
- optional outside-the-cluster checks for user-facing reachability

## What This Should Not Try To Do

This path should stay small.

It should not try to prove:

- GitOps import
- controller-managed sync
- layered recipe composition
- fleet variants
- policy or scan workflows

Those all matter, but they are follow-on stories. The value of this path is that it proves a real cluster can come online, surface a usable target, and run a basic workload.

## Relationship To Current Examples

This is a strong bridge between:

- [vmcluster-from-scratch](./vmcluster-from-scratch.md)
- the incubator GitOps import examples
- the direct deployment variants in [global-app-layer](./global-app-layer/README.md)

It is also a strong future `cub-proc` candidate because it is a bounded live procedure with a clean handoff and concrete verification.

If you want to run the real implementation, use:

- [`jesperfj/cub-vmcluster`](https://github.com/jesperfj/cub-vmcluster)

See:

- [cub-proc/vmcluster-bootstrap-profile.md](./cub-proc/vmcluster-bootstrap-profile.md)
- [cub-proc/vmcluster-nginx-deploy-profile.md](./cub-proc/vmcluster-nginx-deploy-profile.md)
