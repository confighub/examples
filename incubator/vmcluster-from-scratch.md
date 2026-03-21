# `cub-vmcluster` From Scratch

This note is the simplest way to think about `cub-vmcluster` from a user point of view.

The user story is not mainly about workers. It is about getting a real Kubernetes cluster somewhere inexpensive, connecting it to ConfigHub, and then using it as a deployment target for workloads.

## The Mental Model

Think about it in this order:

1. create or load a cluster config
2. boot the cluster
3. let the cluster connect itself to ConfigHub
4. wait for a target to appear
5. deploy workloads to that target

That is the important user-facing shape.

In other words:

- first you get a cluster
- then ConfigHub becomes connected to it
- then you can deploy apps to it

## What The User Cares About

Most users care about two things:

- a real running cluster
- a target they can deploy to

Most users do not want to think about workers unless they are troubleshooting.

That means the public docs should present the worker as implementation detail most of the time and present the target as the main live-system object that app and deployment examples bind to.

## What Happens Today

Today the implementation uses the worker and target model that ConfigHub already has.

The rough flow is:

1. a cluster config says which worker name and space to use
2. the cluster boots
3. the cluster creates or reuses that worker
4. the worker connects to ConfigHub
5. the worker creates or reuses a target
6. workload units are pointed at that target

The practical result is that you should expect a target to show up with a name like:

```text
<worker-name>-kubernetes-yaml-cluster
```

That is the thing workload units should bind to.

## Why This Matters

This is the first example path that makes the worker and target model feel more real instead of theoretical.

It gives us:

- real clusters in real regions
- a more believable target model
- a cleaner bridge from ConfigHub examples to live deployment
- a better place to pressure-test worker and target UX

## How It Fits With The Current Examples

`cub-vmcluster` is not the current front door. The current wedge is still:

- GitHub
- Argo or Flux
- AI or CLI
- ConfigHub
- evidence

But `cub-vmcluster` is a strong next bridge from that wedge into real live targets.

Good follow-on pairings are:

- a simple nginx workload
- the GitOps import demos
- the direct deployment variants in `global-app-layer`

## How To Talk About It In Docs

When describing this flow, prefer:

- cluster first
- target second
- worker as implementation detail

Only surface the worker explicitly when:

- the user is setting the initial cluster config
- the worker failed to connect
- the expected target did not appear

## Relationship To `cub-proc`

This is a strong future `cub-proc` candidate because it is naturally a bounded procedure:

- preflight cloud credentials
- create or load the cluster config
- apply the cluster definition
- wait for worker connection
- assert target readiness
- return the target ref for later deployment

See:

- [cub-proc/vmcluster-bootstrap-profile.md](./cub-proc/vmcluster-bootstrap-profile.md)
- [vmcluster-nginx-path.md](./vmcluster-nginx-path.md)
