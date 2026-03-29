# Why ConfigHub Should Have an OCI Distribution API

## Summary

ConfigHub should expose a read-only OCI Distribution API.

This would make ConfigHub the **origin** for controller-oriented bundle delivery:

- **Direct Kubernetes** remains the simplest real apply path
- **Flux OCI** becomes the standard controller path without forcing a separate registry
- **Argo OCI** gets the same clean model
- external OCI registries stay optional for caching, mirroring, compliance, or air-gap workflows

The point is not to turn ConfigHub into "yet another registry product".
The point is to let Flux, Argo, Helm, `oras`, `crane`, and `skopeo` talk to ConfigHub through a protocol they already understand.

## The Problem

### Flux and Argo Need OCI

Modern controller-oriented delivery has converged on OCI as the transport layer:

- **Flux** consumes OCI artifacts through `OCIRepository`
- **Argo CD 3.1+** can use OCI as an application source
- **Helm** now treats OCI as a standard distribution path

So when a ConfigHub user wants controller-driven delivery instead of direct `kubectl apply`, OCI is no longer optional in practice.

### ConfigHub Needs an Out-of-the-Box Option

Today the direct Kubernetes story is simple:

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

If ConfigHub can already offer an out-of-the-box direct apply story, it should also be able to offer an out-of-the-box OCI origin for Flux and Argo.

### ConfigHub Is Already Authoritative

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

## The Proposal

### OCI Should Be a Gateway, Not a Separate System

ConfigHub should expose the OCI Distribution API as another protocol surface over deployment output.

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

### What ConfigHub Would Serve

ConfigHub should expose a stable OCI view of the **deployment output for a specific deployment variant and target path**.

That identity should be deployment-oriented, not just storage-oriented.

The important user-facing identity is:

- which deployment variant this is
- which target or delivery mode it is for
- which exact revision or digest it represents

Internally, ConfigHub can still map that to spaces, units, revisions, and stored data. But the proposal should model the OCI artifact as a deployment artifact, not as a raw database row.

### Start With Read-Only Distribution

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

Do **not** make push support part of the first proposal.

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

ConfigHub already owns the approval and provenance context.

If ConfigHub also serves the OCI artifact, then the operator can inspect one system for:

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

The important point is that ConfigHub should be able to feed those systems cleanly, not require them as the only starting point.

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

- **shared identity system where possible**

not:

- **the exact same creds everywhere**

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

That gives ConfigHub a cleaner controller-oriented path for:

- OCIRepository
- Kustomization
- HelmRelease-backed flows

### Argo

Argo can use the same origin model once OCI-backed Application sources are in play.

That gives ConfigHub a cleaner Argo story than renderer-only workflows:

- publish deployment output
- point Argo at the OCI source
- let Argo reconcile workloads from that source

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

The first cut should **not** require:

- write/push support
- replacing every external registry workflow
- inventing a new bundle product model separate from ConfigHub deployment state

## Conclusion

Flux and Argo need OCI.
ConfigHub needs an out-of-the-box controller path.
ConfigHub is already authoritative for approved deployment state.

So ConfigHub should expose OCI directly.

That makes OCI a transport and gateway layer over ConfigHub's authoritative deployment output.

It keeps the default path simple:

- direct Kubernetes when you want the smallest real proof
- Flux OCI when you want the standard controller path
- Argo OCI when you want the matching Argo controller path

And it still leaves room for Spegel, mirrors, and third-party registries where they create value.
