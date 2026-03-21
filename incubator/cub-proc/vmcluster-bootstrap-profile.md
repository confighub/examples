# `cub-proc` Profile Draft: `vmcluster/bootstrap`

This document is a first draft of a `cub-proc` profile for bringing up a `cub-vmcluster` cluster and making it usable as a deployment target.

The point of the profile is not to hide the current implementation. The point is to give one bounded operational record for the whole bootstrap flow instead of leaving it split across shell scripts, cloud state, worker logs, and ConfigHub screens.

## Procedure Name

```text
vmcluster/bootstrap
```

## Goal

Turn a cluster config into:

- a running cluster
- a connected ConfigHub worker
- a ready target that later workload examples can bind to

The key usable output of the procedure is not the worker. It is the target reference.

## Subject

The subject is a cluster definition or cluster config package for `cub-vmcluster`.

Examples:

- one cluster config unit already stored in ConfigHub
- one package or directory that can be loaded into ConfigHub first

## Typical Inputs

- cluster name
- ConfigHub space for the worker
- worker name
- cloud credentials and region
- instance class
- optional DNS or ingress settings

## Main Output

- target ref, typically:

```text
<worker-name>-kubernetes-yaml-cluster
```

Optional additional outputs:

- cluster endpoint
- kubeconfig location or retrieval hint
- ingress base domain

## Top-Level Phases

### 1. `preflight`

Check:

- authenticated ConfigHub context
- required cloud credentials
- expected region and quota assumptions
- whether the chosen worker space exists
- whether the requested worker name is available or intentionally reused

### 2. `materialize-cluster-config`

Create, load, or resolve the cluster config that `cub-vmcluster` will act on.

This phase ends when the desired cluster definition is ready to apply.

### 3. `apply-cluster-config`

Apply the cluster definition through the `cub-vmcluster` path.

This phase is about initiating cluster creation, not proving the cluster is usable yet.

### 4. `wait-for-worker`

Wait for the cluster-created worker to connect back to ConfigHub.

This is where the procedure first proves that ConfigHub has a live relationship with the new cluster.

### 5. `assert-target-ready`

Wait for the worker-created target to exist and be usable.

This is the real handoff point for later workload deployment.

### 6. `summarize`

Return:

- target ref
- worker name
- cluster name
- any useful URLs or kubeconfig hints

## Assertions

Required assertions:

- ConfigHub auth is valid
- cluster config exists or was created successfully
- worker connected
- target exists
- target is usable for direct Kubernetes apply

Optional assertions:

- cluster API is reachable
- ingress is reachable
- TLS is configured
- a smoke workload such as nginx is reachable

## Done Semantics

The procedure should be `done` when:

- the cluster bootstrap work has completed
- the worker is connected
- the target exists and is ready for later workload binding

The procedure does not need to prove that a later app deployment has succeeded. That is a separate procedure.

## Failure Semantics

Common failure classes:

- cloud preflight failure
- cluster creation failure
- worker created but never connected
- worker connected but target never appeared
- target exists but is not yet usable

These should be visible as step or assertion failures, not hidden in logs.

## Evidence Model

This profile would need evidence from at least three places:

- ConfigHub facts
  - worker exists
  - target exists
  - target metadata is visible
- cluster facts
  - API reachable
  - optional system pods healthy
- infrastructure facts
  - instance or cluster bootstrap succeeded

The profile should prefer returning the target ref as the main success artifact because that is what downstream deployment examples consume.

## Why This Profile Matters

This is a stronger live-system bootstrap than local kind-based examples.

It helps the product in three ways:

- it gives a real first-cluster story
- it makes the worker and target model more concrete
- it provides a realistic bridge from ConfigHub examples to live deployment

## Relationship To Current Examples

This profile is a good upstream procedure for:

- a simple nginx workload
- `gitops-import-argo`
- `gitops-import-flux`
- direct deployment variants in `global-app-layer`

That makes it a useful procedure candidate even before the full `cub-proc` system exists.
