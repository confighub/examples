# OCI Distribution Architecture: Zot + Spegel (Historical Exploration)

> **Status**: ConfigHub now exposes a native OCI Distribution API. This document is preserved as historical exploration of an alternative architecture. It is **not the recommended approach**.
>
> The standard delivery path is:
> ```
> ConfigHub-native OCI origin -> Flux/Argo -> cluster
> ```
>
> External registries, caches, and mirrors (including Zot, Spegel, Harbor, etc.) remain optional for specific needs like air-gap workflows, regional caching, or compliance requirements.

This document explored an OCI artifact distribution architecture before ConfigHub's native OCI API existed.

## Historical Problem Statement

Before ConfigHub had native OCI support, the architecture required:

1. ConfigHub server pushes to an external OCI registry (e.g., GHCR, ECR)
2. Flux/Argo controllers pull from that registry
3. Each cluster independently pulls the same artifacts

This created:
- External registry dependency
- Network egress costs
- Rate limiting exposure
- Latency for distributed clusters

**This problem is now solved by ConfigHub's native OCI API.** The exploration below remains for reference on caching and distribution layers that may still be useful for large-scale deployments.

## Historical Proposed Architecture (Superseded)

```
                    ┌─────────────────────┐
                    │   ConfigHub Server  │
                    │        + Zot        │
                    │  (primary registry) │
                    └─────────┬───────────┘
                              │
              ┌───────────────┼───────────────┐
              │               │               │
              ▼               ▼               ▼
        ┌───────────┐   ┌───────────┐   ┌───────────┐
        │  Cluster  │   │  Cluster  │   │  Cluster  │
        │   + cub   │   │   + cub   │   │   + cub   │
        │ + Spegel* │   │ + Spegel* │   │ + Spegel* │
        │   + Flux  │   │   + Argo  │   │   + Flux  │
        └───────────┘   └───────────┘   └───────────┘

        Spegel* = Extended Spegel with OCI artifact support
```

### Component Roles

**Zot (Central)**
- Co-located with ConfigHub server
- Receives bundle pushes from `cub unit apply` via oras
- Single-binary, OCI-native, minimal footprint
- Serves as authoritative artifact source

**Extended Spegel (Distributed)**
- Runs alongside cub workers on each cluster
- Caches OCI artifacts from central Zot
- P2P distribution between nodes (Kademlia DHT)
- Serves local Flux/Argo controllers

## Spegel Extension Requirements

Current Spegel limitations:
- Only intercepts containerd image pulls
- Queries containerd content store for available layers
- Cannot serve OCI artifacts consumed via HTTP/oras

Required extensions:

### 1. HTTP Registry Proxy Mode

Spegel must expose an HTTP endpoint that Flux/Argo can use as their OCI source:

```yaml
# Flux OCIRepository pointing to local Spegel
apiVersion: source.toolkit.fluxcd.io/v1beta2
kind: OCIRepository
metadata:
  name: my-app
spec:
  url: oci://spegel.spegel-system.svc.cluster.local/confighub/my-app
  # Instead of: oci://zot.confighub.example.com/confighub/my-app
```

### 2. Artifact Content Store

Spegel needs its own content store for OCI artifacts (separate from containerd):

```
/var/lib/spegel/artifacts/
├── blobs/
│   └── sha256/
│       ├── abc123...  # manifest
│       └── def456...  # layer
└── index/
    └── confighub/
        └── my-app/
            └── latest -> sha256:abc123...
```

### 3. Upstream Fetch

When artifact not in local cache:
1. Query DHT for peer with content
2. If peer found: fetch from peer
3. If no peer: fetch from upstream Zot
4. Cache locally and advertise to DHT

### 4. DHT Advertisement

Advertise artifact digests to DHT same as container images:
- State tracker periodically scans artifact store
- Announces available digests with configurable TTL

## Implementation Approach

### Option A: Extend Spegel Upstream

Contribute to spegel-org/spegel:

Pros:
- Community maintained
- Broader testing
- Aligned with existing P2P infrastructure

Cons:
- Maintainer concern about scope creep (see [issue #821](https://github.com/spegel-org/spegel/issues/821))
- May conflict with Spegel's "stateless" philosophy
- Longer timeline for acceptance

### Option B: ConfigHub Artifact Proxy (Recommended)

Build a purpose-built component inspired by Spegel:

```
confighub-artifact-proxy (cap)
├── registry/           # OCI distribution API server
├── store/              # Local artifact content store
├── dht/                # Kademlia DHT client (reuse spegel/pkg/routing)
├── upstream/           # Zot client for cache misses
└── state/              # Artifact state tracker
```

Pros:
- Purpose-built for ConfigHub use case
- Can reuse Spegel's DHT implementation
- No upstream negotiation
- Faster iteration

Cons:
- Additional component to maintain
- Duplicates some Spegel functionality

### Hybrid: Fork + Contribute Back

1. Fork Spegel
2. Add artifact support for ConfigHub needs
3. Propose generic changes back upstream
4. Maintain ConfigHub-specific features in fork

## Deployment Model

### Central (ConfigHub Server)

```yaml
# docker-compose or k8s deployment
services:
  confighub-server:
    # existing server
  zot:
    image: ghcr.io/project-zot/zot-linux-amd64:latest
    ports:
      - "5000:5000"
    volumes:
      - zot-data:/var/lib/registry
    environment:
      - ZOT_HTTP_ADDRESS=0.0.0.0
      - ZOT_HTTP_PORT=5000
```

### Distributed (Per Cluster)

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: confighub-artifact-proxy
  namespace: confighub-system
spec:
  template:
    spec:
      containers:
        - name: cap
          image: ghcr.io/confighub/artifact-proxy:latest
          args:
            - --upstream=https://zot.confighub.example.com
            - --listen=:5001
            - --store=/var/lib/cap
          ports:
            - containerPort: 5001
          volumeMounts:
            - name: store
              mountPath: /var/lib/cap
      volumes:
        - name: store
          emptyDir: {}
```

## Security Considerations

1. **Authentication**: Proxy must forward credentials to upstream Zot
2. **TLS**: All inter-node traffic should use mTLS
3. **Content Verification**: Verify digests on fetch and serve
4. **Network Policy**: Restrict proxy egress to known upstreams

## Migration Path

### Phase 1: Zot Only
- Deploy Zot alongside ConfigHub server
- Configure `cub` to push to local Zot
- Flux/Argo pull directly from Zot

### Phase 2: Add Caching Proxy
- Deploy artifact proxy on clusters
- Configure Flux/Argo to use local proxy
- Proxy forwards to Zot on cache miss

### Phase 3: Enable P2P
- Enable DHT between proxies
- Artifacts replicate across clusters
- Reduced load on central Zot

## Related Work

- [Spegel](https://spegel.dev/) - P2P container image distribution
- [Zot](https://zotregistry.dev/) - OCI-native registry
- [Spegel Issue #821](https://github.com/spegel-org/spegel/issues/821) - OCI artifact sideloading
- [Spegel PR #977](https://github.com/spegel-org/spegel/pull/977) - Push support

## Open Questions

1. Should the artifact proxy handle container images too, or leave that to standard Spegel?
2. What's the TTL for cached artifacts before re-validation?
3. How do we handle artifact garbage collection across the P2P network?
4. Should we support artifact replication factor (n copies across cluster)?
