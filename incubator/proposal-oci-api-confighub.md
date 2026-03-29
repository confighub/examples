# ConfigHub Native OCI Distribution API

> **Status**: The core OCI Distribution API surface is now implemented in ConfigHub. This document preserves the original proposal rationale and describes the remaining documentation, productization, and proof gaps. Treat this as design context and gap tracking, not as a request for a missing feature.

## Current Status

ConfigHub now exposes a native OCI Distribution API. The core capability exists:

- Flux and Argo can consume ConfigHub as an OCI origin without requiring a separate registry
- The standard controller path is: `ConfigHub-native OCI origin -> Flux/Argo -> cluster`
- External registries remain optional for caching, mirroring, compliance, or air-gap workflows

**Remaining gaps** (documentation and proof, not core capability):
- User-facing OCI documentation in main ConfigHub docs (not yet merged)
- Full end-to-end proof in these incubator examples showing: `ConfigHub revision -> OCI ref/digest -> controller source -> live workload evidence`
- Productization polish (error messages, CLI discoverability, GUI evidence views)

The sections below preserve the original proposal rationale for context.

---

## Original Proposal Summary

ConfigHub exposes a read-only OCI Distribution API.

That would make ConfigHub the origin for controller-oriented bundle delivery:

- Flux and Argo get an out-of-the-box OCI path without requiring a separate registry first
- customers can still connect external OCI registries and tools for caching, mirroring, compliance, or air-gap workflows
- direct worker apply to Kubernetes remains available where that is the better fit

OCI is the transport here, not a separate product category. This should also help standard tools such as `oras`, `crane`, `skopeo`, and Helm.

## Flux And Argo Need OCI

Modern controller-oriented delivery has converged on OCI transport:

- Flux consumes OCI artifacts through `OCIRepository`
- Argo CD 3.1+ can use OCI as an application source
- Helm now treats OCI as a standard distribution path

So when a ConfigHub user wants controller-driven delivery instead of direct `kubectl apply`, OCI is no longer optional in practice.

## ConfigHub Provides an Out-of-the-Box Option

The direct Kubernetes story is simple:

```text
ConfigHub -> worker -> cluster
```

But the controller-oriented story usually becomes:

```text
ConfigHub -> publish/sync -> external registry -> Flux/Argo -> cluster
```

That adds a second operational system before the user has even proven the deployment flow:

- registry provisioning
- registry auth and secrets
- network access from clusters
- storage and availability concerns
- sync or publication drift between ConfigHub and the registry

That is too much burden for the default path.

ConfigHub offers both an out-of-the-box direct apply story and an out-of-the-box OCI origin for Flux and Argo. Third-party OCI technology remains optional, not mandatory.

## ConfigHub Is Authoritative

ConfigHub is already the authoritative store for:

- approved intended state
- deployment variants
- target bindings
- revision history
- provenance and governance context

For controller-oriented delivery, the external registry is usually just another place to publish the same approved deployment output.

That creates avoidable complexity:

- one system decides what should be deployed
- another system becomes the place controllers must read from
- operators then have to prove the two still match

The better default is:

```text
ConfigHub (authoritative) -> OCI API -> Flux/Argo
```

not:

```text
ConfigHub (authoritative) -> copy/sync -> registry -> Flux/Argo
```

## OCI Is a Gateway, Not a Separate System

ConfigHub exposes the OCI Distribution API as another protocol surface over deployment output.

Conceptually:

```text
                +----------------------+
                |      ConfigHub       |
                | authoritative state  |
                +----------+-----------+
                           |
        +------------------+------------------+
        |                  |                  |
        v                  v                  v
      cub/CLI           GUI/API           OCI API
   operator access   human inspection   controller access
```

`cub` already reaches ConfigHub through one interface. The GUI reaches it through another. Flux, Argo, and OCI tooling should be able to do the same through `/v2/...`.

## What ConfigHub Serves

ConfigHub exposes a stable OCI view of the deployment output for a specific deployment variant and target path.

That identity should be deployment-oriented, not just storage-oriented. The important user-facing identity is:

- which deployment variant this is
- which target or delivery mode it is for
- which exact revision or digest it represents

Internally, ConfigHub can still map that to spaces, units, revisions, and stored data. The proposal should describe the OCI artifact as a deployment artifact, not as a raw database row.

## Start With Read-Only Distribution

The first milestone should be a read-only OCI Distribution API.

That is enough for:

- Flux OCI consumption
- Argo OCI consumption
- OCI inspection with standard tools
- mirroring to third-party registries

Minimum endpoints:

- `GET /v2/`
- `GET /v2/<repo>/tags/list`
- `HEAD /v2/<repo>/manifests/<ref>`
- `GET /v2/<repo>/manifests/<ref>`
- `HEAD /v2/<repo>/blobs/<digest>`
- `GET /v2/<repo>/blobs/<digest>`

Do not make push support part of the first proposal.

`oras push` or `crane push` creating ConfigHub units raises separate product questions about:

- approval and governance
- labels and metadata
- ancestry and clone links
- recipe manifests
- App-Deployment-Target mapping

Those should be treated as a later design packet, not folded into the initial OCI API decision.

## Why This Is Better Than External Registry First

### Better Default Experience

With a ConfigHub OCI origin, the default controller path becomes:

```text
ConfigHub -> OCI API -> Flux/Argo -> cluster
```

instead of:

```text
ConfigHub -> external registry -> Flux/Argo -> cluster
```

That removes mandatory extra infrastructure from the first successful run.

### Better Consistency Story

The standard proof chain becomes:

```text
ConfigHub revision -> OCI digest -> controller source -> live workload
```

That is much easier to reason about than:

```text
ConfigHub revision -> sync job -> registry tag -> controller source -> live workload
```

### Better Audit Story

ConfigHub already owns the approval and provenance context. If it also serves the OCI artifact, the operator can inspect one system for:

- what was approved
- what digest was published
- which target path it was for
- which controller consumed it

That is cleaner than correlating ConfigHub state with a second registry system from the start.

## Delivery Model

### Small And Simple

For small deployments, controllers can point straight at ConfigHub:

```text
Flux/Argo -> ConfigHub OCI origin
```

This is the out-of-the-box path.

### Cached And Distributed

For larger or more distributed setups, ConfigHub remains the OCI origin and a cache sits in front.

For example:

```text
ConfigHub OCI origin -> Spegel or similar cache -> Flux/Argo
```

In this model, Spegel is not the source of truth. It is the CDN or cache layer:

- local caching near clusters
- lower load on the origin
- better distribution across many nodes or regions
- resilience during temporary origin interruption

### Bridge To Third-Party Registries

Some users will still need a corporate OCI registry for policy, compliance, or air-gap workflows.

That should remain optional:

```text
ConfigHub OCI origin -> crane/oras/skopeo or native replication -> third-party registry
```

This keeps one authoritative origin while still supporting:

- Harbor
- GHCR
- ECR
- ACR
- disconnected or staged environments

ConfigHub should be able to feed those systems cleanly. They should not be required as the only starting point.

## Identity, Auth, And Proof

### Identity

The artifact identity should be deployment-aware.

The proposal should describe an OCI repository as representing the output for a deployment path, not merely "space plus unit". That matches the App-Deployment-Target direction and the bundle walkthroughs already used in these examples.

### Authentication

ConfigHub should reuse its existing identity system, but the proposal should not pretend that one literal credential fits every case.

We need to distinguish:

- human or AI operator auth
- controller pull auth for Flux or Argo
- service-account or robot credentials for cluster-side pulls
- optional mirror credentials for downstream registries

So the right claim is:

- shared identity system where possible

not:

- the exact same creds everywhere

### Proof Standard

The default proof should be digest-based, not `latest`-based.

For a real end-to-end claim, the evidence should show:

1. the ConfigHub revision
2. the OCI digest served for that revision
3. the controller resource pointing at that digest or the digest-backed publication
4. the live workload delivered from that same source

That is the standard both Flux OCI and Argo OCI should meet.

## What This Enables

### Flux

Flux can consume ConfigHub output without requiring a separate registry as the first step.

That gives ConfigHub a straightforward controller-oriented path for `OCIRepository`, `Kustomization`, and HelmRelease-backed flows.

### Argo

Argo can use the same origin model through OCI-backed Application sources.

That gives ConfigHub a simpler Argo story than renderer-only workflows: publish deployment output, point Argo at the OCI source, and let Argo reconcile workloads from that source.

### Standard OCI Tooling

Once ConfigHub exposes OCI directly, standard tools become available without special adapters:

- `oras`
- `crane`
- `skopeo`

That matters for:

- inspection
- mirroring
- air-gap staging
- evidence capture

## Recommended First Scope

The first cut should be:

1. read-only OCI Distribution API
2. deployment-oriented repository identity
3. digest-first proof model
4. direct controller consumption by Flux and Argo
5. optional cache or mirror layers in front of or downstream from ConfigHub

The first cut should not require:

- write or push support
- replacing every external registry workflow
- inventing a new bundle product model separate from ConfigHub deployment state

## Conclusion

Flux and Argo need OCI.
ConfigHub provides an out-of-the-box controller path.
ConfigHub is authoritative for approved deployment state.

ConfigHub exposes OCI directly.

OCI is a transport and gateway layer over ConfigHub's authoritative deployment output.

The default path is simple:

- direct Kubernetes when you want the smallest real proof
- Flux OCI when you want the standard controller path
- Argo OCI when you want the matching Argo controller path

And it still leaves room for Spegel, mirrors, and third-party registries where they create value.
