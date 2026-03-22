# `cub-proc` Profile Draft: `vmcluster/nginx-deploy`

This document is a first draft of a `cub-proc` profile for the smallest useful live deployment after `cub-vmcluster` bootstrap.

The point of the profile is to capture one bounded path from ready target to reachable workload. That makes it a good first deployment procedure before moving into GitOps import or larger app examples.

## Procedure Name

```text
vmcluster/nginx-deploy
```

## Goal

Turn:

- one ready `cub-vmcluster` target
- one small nginx workload definition

into:

- a bound deployment unit
- a successful direct apply
- verified runtime evidence

The key usable output is not just that apply was requested. It is that the workload can be proven live on the target cluster.

## Subject

The subject is a small workload package or unit set intended for direct Kubernetes deployment.

Typical shape:

- namespace
- deployment
- service
- optional ingress

## Typical Inputs

- target ref
- namespace
- workload name
- image reference
- optional hostname
- optional TLS or ingress settings

## Main Output

- confirmed target ref
- workload namespace
- workload name
- service or ingress endpoint
- optional public URL

## Top-Level Phases

### 1. `preflight`

Check:

- authenticated ConfigHub context
- target ref is present
- target is compatible with direct Kubernetes apply
- workload package or units exist

### 2. `bind-target`

Bind the workload deploy unit to the chosen target.

This phase is complete when the unit metadata clearly points to the target that `vmcluster/bootstrap` produced.

### 3. `apply`

Apply the workload through the direct Kubernetes path.

This phase is about sending the intended workload to the target, not proving full runtime success yet.

### 4. `assert-cluster-state`

Verify the raw cluster facts:

- namespace exists
- deployment exists
- deployment is available
- service exists
- ingress exists if requested

### 5. `assert-reachability`

If the workload exposes a reachable endpoint, verify:

- service responds through port-forward or cluster access
- ingress hostname resolves if used
- HTTPS responds if TLS is configured

### 6. `summarize`

Return:

- target ref
- namespace
- deployment name
- service name
- optional ingress URL

## Assertions

Required assertions:

- target exists
- target is ready for direct apply
- workload unit is bound to the target
- deployment is available
- service exists

Optional assertions:

- ingress exists
- hostname resolves
- HTTPS is valid
- expected response body is returned

## Done Semantics

The procedure should be `done` when:

- the workload is bound to the target
- apply has completed
- the raw cluster evidence shows the deployment is live

If a URL or ingress path is part of the intended outcome, the stronger done condition is a reachable response.

## Failure Semantics

Common failure classes:

- target missing or incompatible
- apply failed
- deployment created but pods never became ready
- service exists but no endpoints are serving
- ingress or DNS not ready
- TLS not configured or invalid

These should be visible as step or assertion failures, not hidden in controller logs or shell output.

## Evidence Model

This profile would need evidence from at least three places:

- ConfigHub facts
  - target ref
  - unit binding
  - apply request and result
- cluster facts
  - namespace
  - deployment status
  - service
  - ingress
- edge facts
  - resolved hostname
  - HTTP or HTTPS response

This is the smallest live deployment profile that still proves something meaningful about the target and the workload.

## Why This Profile Matters

This profile is useful because it keeps the live deployment story small and concrete.

It helps the product in three ways:

- it proves that a real cluster target can actually run a workload
- it gives a simple handoff after `vmcluster/bootstrap`
- it creates a cleaner baseline before larger deployment stories

## Relationship To Other Profiles

This profile should normally depend on:

- [vmcluster-bootstrap-profile.md](./vmcluster-bootstrap-profile.md)

It is also a good precursor for:

- [../gitops-import-argo](../gitops-import-argo/README.md)
- [../gitops-import-flux](../gitops-import-flux/README.md)
- [../global-app-layer](../global-app-layer/README.md)
